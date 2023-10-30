// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/handshake"
)

const (
	tokenByteSize = 32
	NoopToken     = "noop"
)

const (
	handShakeTypeUID   = "UID"
	handShakeTypeTOKEN = "TOKEN"
)

const (
	expirationTime = 30 * time.Minute
)

type Token struct {
	Token   string
	Version int64
}

type AfterRegisterDomainHook func(response *handshake.RegisterResponse)

func (c *DomainRouteController) startHandShakeServer(port uint32) {
	mux := http.NewServeMux()
	mux.HandleFunc("/handshake", c.handShakeHandle)
	if c.masterConfig != nil && c.masterConfig.Master {
		mux.HandleFunc("/register", c.registerHandle)
	}

	c.handshakeServer = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	nlog.Error(c.handshakeServer.ListenAndServe())
}

func doHTTP(in interface{}, out interface{}, path, host string, headers map[string]string) error {
	maxRetryTimes := 5

	for i := 0; i < maxRetryTimes; i++ {
		inbody, err := json.Marshal(in)
		if err != nil {
			nlog.Errorf("new handshake request fail:%v", err)
			return err
		}
		req, err := http.NewRequest(http.MethodPost, config.InternalServer+path, bytes.NewBuffer(inbody))
		if err != nil {
			nlog.Errorf("new handshake request fail:%v", err)
			return err
		}
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		for key, val := range headers {
			req.Header.Set(key, val)
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			nlog.Errorf("do http request fail:%v", err)
			time.Sleep(time.Second)
			continue
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			nlog.Warnf("Request error, path: %s, code: %d, message: %s", path, resp.StatusCode, string(body))
			time.Sleep(time.Second)
			continue
		}
		if err := json.Unmarshal(body, out); err != nil {
			nlog.Errorf("Json unmarshal failed, err:%s, body:%s", err.Error(), string(body))
			time.Sleep(time.Second)
			continue
		}
		return nil
	}

	return fmt.Errorf("request error, retry at maxtimes:%d, path: %s", maxRetryTimes, path)
}

func (c *DomainRouteController) waitTokenReady(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	maxRetryTimes := 60
	i := 0
	t := time.NewTicker(time.Second)
	for {
		i++
		if i == maxRetryTimes {
			return fmt.Errorf("do http request fail, at max retry times:%d", maxRetryTimes)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			req, err := http.NewRequest(http.MethodGet, config.InternalServer+"/handshake", nil)
			if err != nil {
				nlog.Errorf("new handshake request fail:%v", err)
				return err
			}
			ns := dr.Spec.Destination
			if dr.Spec.Transit != nil {
				ns = dr.Spec.Transit.Domain.DomainID
			}
			req.Host = fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(fmt.Sprintf("%s-Cluster", clusters.ServiceHandshake), fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, ns, dr.Spec.Endpoint.Ports[0].Name))
			req.Header.Set("Kuscia-Source", dr.Spec.Source)
			req.Header.Set("kuscia-Host", fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns))
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				nlog.Errorf("do http request fail:%v, retry:%d", err, i)
				continue
			}

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read body failed, err:%s, code: %d", err.Error(), resp.StatusCode)
			}
			if resp.StatusCode != http.StatusOK {
				if len(body) > 200 {
					body = body[:200]
				}
				nlog.Errorf("Request error(%d), path: %s, code: %d, message: %s", i, fmt.Sprintf("%s.%s.svc/handshake", clusters.ServiceHandshake, ns), resp.StatusCode, string(body))
				continue
			}
			out := &getResponse{}
			if err := json.Unmarshal(body, out); err == nil && out.TokenReady {
				dr = dr.DeepCopy()
				dr.Status.TokenStatus.RevisionToken.IsReady = true
				_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr, metav1.UpdateOptions{})
				return err
			}
		}
	}
}

func (c *DomainRouteController) sourceInitiateHandShake(dr *kusciaapisv1alpha1.DomainRoute) error {
	if dr.Spec.TokenConfig.SourcePublicKey != c.gateway.Status.PublicKey {
		nlog.Errorf("DomainRoute %s: mismatch source public key", dr.Name)
		return nil
	}

	handshankeReq := &handshake.HandShakeRequest{
		DomainId:    dr.Spec.Source,
		RequestTime: time.Now().UnixNano(),
	}

	//1. In UID mode, the token is directly generated by the peer end and encrypted by the local public key
	//2. In RSA mode, the local end and the peer end generate their own tokens and concatenate them.
	//   The local token is encrypted with the peer's public key and then sent.
	//   The peer token is encrypted with the local public key and returned.
	var token []byte
	resp := &handshake.HandShakeResponse{}
	if dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenUIDRSA {
		handshankeReq.Type = handShakeTypeUID
		ns := dr.Spec.Destination
		if dr.Spec.Transit != nil {
			ns = dr.Spec.Transit.Domain.DomainID
		}
		headers := map[string]string{
			fmt.Sprintf("%s-Cluster", clusters.ServiceHandshake): fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, ns, dr.Spec.Endpoint.Ports[0].Name),
			"Kuscia-Source": dr.Spec.Source,
			"kuscia-Host":   fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns),
		}
		err := doHTTP(handshankeReq, resp, "/handshake", fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns), headers)
		if err != nil {
			nlog.Errorf("DomainRoute %s: handshake fail:%v", dr.Name, err)
			return err
		}

		token, err = decryptToken(c.prikey, resp.Token.Token, tokenByteSize)
		if err != nil {
			return err
		}
	} else if dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA {
		handshankeReq.Type = handShakeTypeTOKEN
		sourceToken := make([]byte, tokenByteSize/2)
		_, err := rand.Read(sourceToken)
		if err != nil {
			return err
		}

		//Resolve the public key of the peer end from domainroute crd
		destPub, err := base64.StdEncoding.DecodeString(dr.Spec.TokenConfig.DestinationPublicKey)
		if err != nil {
			nlog.Errorf("DomainRoute %s: destination public key format error, must be base64 encoded", dr.Name)
			return err
		}
		destPubKey, err := tlsutils.ParsePKCS1PublicKey(destPub)
		if err != nil {
			return err
		}
		sourceTokenEnc, err := encryptToken(destPubKey, sourceToken)
		if err != nil {
			return err
		}
		handshankeReq.TokenConfig = &handshake.TokenConfig{
			Token:    sourceTokenEnc,
			Revision: dr.Status.TokenStatus.RevisionToken.Revision,
		}

		ns := dr.Spec.Destination
		if dr.Spec.Transit != nil {
			ns = dr.Spec.Transit.Domain.DomainID
		}
		headers := map[string]string{
			fmt.Sprintf("%s-Cluster", clusters.ServiceHandshake): fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, ns, dr.Spec.Endpoint.Ports[0].Name),
			"Kuscia-Source": dr.Spec.Source,
			"kuscia-Host":   fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns),
		}

		err = doHTTP(handshankeReq, resp, "/handshake", fmt.Sprintf("%s.%s.svc", clusters.ServiceHandshake, ns), headers)
		if err != nil {
			nlog.Errorf("DomainRoute %s: handshake fail:%v", dr.Name, err)
			return err
		}
		destToken, err := decryptToken(c.prikey, resp.Token.Token, tokenByteSize/2)
		if err != nil {
			return err
		}
		token = append(sourceToken, destToken...)
	} else {
		return fmt.Errorf("TokenGenMethod must be %s or %s", kusciaapisv1alpha1.TokenGenUIDRSA, kusciaapisv1alpha1.TokenGenMethodRSA)
	}

	// The final token is encrypted with the local private key and stored in the status of domainroute
	tokenEncrypted, err := encryptToken(&c.prikey.PublicKey, token)
	if err != nil {
		return err
	}

	dr = dr.DeepCopy()
	tn := metav1.Now()
	dr.Status.TokenStatus.RevisionToken.Token = tokenEncrypted
	dr.Status.TokenStatus.RevisionToken.Revision = int64(resp.Token.Revision)
	dr.Status.TokenStatus.RevisionToken.IsReady = false
	dr.Status.TokenStatus.RevisionToken.RevisionTime = tn
	if dr.Spec.TokenConfig.RollingUpdatePeriod == 0 {
		dr.Status.TokenStatus.RevisionToken.ExpirationTime = metav1.NewTime(tn.AddDate(100, 0, 0))
	} else {
		tTx := time.Unix(resp.Token.ExpirationTime/int64(time.Second), resp.Token.ExpirationTime%int64(time.Second))
		dr.Status.TokenStatus.RevisionToken.ExpirationTime = metav1.NewTime(tTx)
	}
	_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr, metav1.UpdateOptions{})
	return err
}

type getResponse struct {
	Namespace  string `json:"namespace"`
	TokenReady bool   `json:"tokenReady"`
}

func (c *DomainRouteController) handShakeHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		resp := &getResponse{
			Namespace:  c.gateway.Namespace,
			TokenReady: false,
		}

		domainID := r.Header.Get("Kuscia-Source")
		drName := common.GenDomainRouteName(domainID, c.gateway.Namespace)
		if dr, err := c.domainRouteLister.DomainRoutes(c.gateway.Namespace).Get(drName); err == nil {
			resp.TokenReady = dr.Status.TokenStatus.RevisionToken.IsReady
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			nlog.Errorf("write handshake response fail, detail-> %v", err)
		}
		return
	}

	req := handshake.HandShakeRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		nlog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	drName := common.GenDomainRouteName(req.DomainId, c.gateway.Namespace)
	dr, err := c.domainRouteLister.DomainRoutes(c.gateway.Namespace).Get(drName)
	if err != nil {
		msg := fmt.Sprintf("DomainRoute %s get error: %v", drName, err)
		nlog.Error(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}
	if !(req.Type == handShakeTypeUID && dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenUIDRSA) &&
		!(req.Type == handShakeTypeTOKEN && dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA) {
		nlog.Errorf("handshake Type(%s) not match domainroute required(%s)", req.Type, dr.Spec.TokenConfig.TokenGenMethod)
		return
	}
	resp, err := c.DestReplyHandshake(&req, dr)
	if err != nil {
		nlog.Errorf("DestReplyHandshake for(%s) fail, detail-> %v", drName, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		nlog.Errorf("encode handshake response for(%s) fail, detail-> %v", drName, err)
	} else {
		nlog.Infof("DomainRoute %s handle success", drName)
	}
}

func (c *DomainRouteController) DestReplyHandshake(req *handshake.HandShakeRequest, dr *kusciaapisv1alpha1.DomainRoute) (*handshake.HandShakeResponse, error) {
	srcPub, err := base64.StdEncoding.DecodeString(dr.Spec.TokenConfig.SourcePublicKey)
	if err != nil {
		return nil, err
	}
	sourcePubKey, err := tlsutils.ParsePKCS1PublicKey(srcPub)
	if err != nil {
		return nil, err
	}
	dstRevisionToken := dr.Status.TokenStatus.RevisionToken

	var token []byte
	var respToken []byte
	if req.Type == handShakeTypeUID {
		// If the token in domainroute is empty or has expired, the token is regenerated.
		// Otherwise, the token is returned
		if dstRevisionToken.Token == "" || time.Since(dstRevisionToken.RevisionTime.Time) > expirationTime {
			respToken = make([]byte, tokenByteSize)
			if _, err = rand.Read(respToken); err != nil {
				return nil, err
			}
		} else {
			respToken, err = decryptToken(c.prikey, dstRevisionToken.Token, tokenByteSize)
			if err != nil {
				return nil, err
			}
		}

		token = respToken
	} else if req.Type == handShakeTypeTOKEN {
		sourceToken, err := decryptToken(c.prikey, req.TokenConfig.Token, tokenByteSize/2)
		if err != nil {
			return nil, err
		}
		respToken = make([]byte, tokenByteSize/2)
		if _, err = rand.Read(respToken); err != nil {
			return nil, err
		}

		token = append(sourceToken, respToken...)
	}

	tokenEncrypted, err := encryptToken(&c.prikey.PublicKey, token)
	if err != nil {
		return nil, err
	}

	if dr.Status.TokenStatus.RevisionToken.Token != tokenEncrypted {
		dr = dr.DeepCopy()
		tn := metav1.Now()
		dr.Status.TokenStatus.RevisionToken.Token = tokenEncrypted
		dr.Status.TokenStatus.RevisionToken.Revision++
		dr.Status.TokenStatus.RevisionToken.RevisionTime = tn
		dr.Status.TokenStatus.RevisionToken.IsReady = false
		if dr.Spec.TokenConfig.RollingUpdatePeriod == 0 {
			dr.Status.TokenStatus.RevisionToken.ExpirationTime = metav1.NewTime(tn.AddDate(100, 0, 0))
		} else {
			dr.Status.TokenStatus.RevisionToken.ExpirationTime = metav1.NewTime(tn.Add(2 * time.Duration(dr.Spec.TokenConfig.RollingUpdatePeriod) * time.Second))
		}
		if _, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
		nlog.Infof("Update domainroute %s status", dr.Name)
	}

	respTokenEncrypted, err := encryptToken(sourcePubKey, respToken)
	if err != nil {
		return nil, err
	}

	return &handshake.HandShakeResponse{
		Token: &handshake.Token{
			Token:          respTokenEncrypted,
			ExpirationTime: dr.Status.TokenStatus.RevisionToken.ExpirationTime.UnixNano(),
			Revision:       int32(dr.Status.TokenStatus.RevisionToken.Revision),
		},
	}, nil
}

func (c *DomainRouteController) parseToken(dr *kusciaapisv1alpha1.DomainRoute, routeKey string) ([]*Token, error) {
	var tokens []*Token
	var err error

	if (dr.Spec.Transit != nil && dr.Spec.BodyEncryption == nil) ||
		(dr.Spec.Transit == nil && dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationMTLS) ||
		(dr.Spec.Transit == nil && dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationNone) {
		tokens = append(tokens, &Token{Token: NoopToken})
		return tokens, err
	}

	if (dr.Spec.Transit == nil && dr.Spec.AuthenticationType != kusciaapisv1alpha1.DomainAuthenticationToken) ||
		dr.Spec.TokenConfig == nil {
		return tokens, fmt.Errorf("invalid DomainRoute: %v", dr.Spec)
	}

	switch dr.Spec.TokenConfig.TokenGenMethod {
	case kusciaapisv1alpha1.TokenGenMethodRSA, kusciaapisv1alpha1.TokenGenUIDRSA:
		tokens, err = c.parseTokenRSA(dr)
	default:
		err = fmt.Errorf("DomainRoute %s unsupported token method: %s", routeKey,
			dr.Spec.TokenConfig.TokenGenMethod)
	}
	return tokens, err
}

func (c *DomainRouteController) parseTokenRSA(dr *kusciaapisv1alpha1.DomainRoute) ([]*Token, error) {
	key, _ := cache.MetaNamespaceKeyFunc(dr)

	var tokens []*Token
	if len(dr.Status.TokenStatus.Tokens) == 0 {
		return tokens, fmt.Errorf("DomainRoute %s has no avaliable token", key)
	}

	if (c.gateway.Namespace == dr.Spec.Source && dr.Spec.TokenConfig.SourcePublicKey != c.gateway.Status.PublicKey) ||
		(c.gateway.Namespace == dr.Spec.Destination && dr.Spec.TokenConfig.DestinationPublicKey != c.gateway.Status.PublicKey) {
		err := fmt.Errorf("DomainRoute %s mismatch public key", key)
		return tokens, err
	}

	for _, token := range dr.Status.TokenStatus.Tokens {
		b, err := decryptToken(c.prikey, token.Token, tokenByteSize)
		if err != nil {
			return []*Token{}, fmt.Errorf("DomainRoute %s decrypt error: %v", key, err)
		}
		tokens = append(tokens, &Token{Token: base64.StdEncoding.EncodeToString(b), Version: token.Revision})
	}

	return tokens, nil
}

func (c *DomainRouteController) checkAndUpdateTokenInstances(dr *kusciaapisv1alpha1.DomainRoute) error {
	if len(dr.Status.TokenStatus.Tokens) == 0 {
		return nil
	}

	updated := false
	dr = dr.DeepCopy()
	for i := range dr.Status.TokenStatus.Tokens {
		if !exists(dr.Status.TokenStatus.Tokens[i].EffectiveInstances, c.gateway.Name) {
			updated = true
			dr.Status.TokenStatus.Tokens[i].EffectiveInstances = append(dr.Status.TokenStatus.Tokens[i].EffectiveInstances, c.gateway.Name)
		}
	}

	if updated {
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr, metav1.UpdateOptions{}); err != nil {
			nlog.Error(err)
			return err
		}
	}
	return nil
}

func encryptToken(pub *rsa.PublicKey, key []byte) (string, error) {
	return tlsutils.EncryptPKCS1v15(pub, key)
}

func decryptToken(priv *rsa.PrivateKey, ciphertext string, keysize int) ([]byte, error) {
	return tlsutils.DecryptPKCS1v15(priv, ciphertext, keysize)
}

func exists(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func HandshakeToMaster(domainID string, prikey *rsa.PrivateKey) error {
	handshankeReq := &handshake.HandShakeRequest{
		DomainId:    domainID,
		RequestTime: time.Now().UnixNano(),
	}

	//1. In UID mode, the token is directly generated by the peer end and encrypted by the local public key
	//2. In RSA mode, the local end and the peer end generate their own tokens and concatenate them.
	//   The local token is encrypted with the peer's public key and then sent.
	//   The peer token is encrypted with the local public key and returned.
	handshankeReq.Type = handShakeTypeUID
	resp := &handshake.HandShakeResponse{}

	headers := map[string]string{
		"Kuscia-Source": domainID,
		fmt.Sprintf("%s-Cluster", clusters.ServiceHandshake): clusters.GetMasterClusterName(),
		"kuscia-Host": fmt.Sprintf("%s.master.svc", clusters.ServiceHandshake),
	}
	err := doHTTP(handshankeReq, resp, "/handshake", fmt.Sprintf("%s.master.svc", clusters.ServiceHandshake), headers)
	if err != nil {
		nlog.Error(err)
		return err
	}

	token, err := decryptToken(prikey, resp.Token.Token, tokenByteSize)
	if err != nil {
		nlog.Error(err)
		return err
	}
	c, err := xds.QueryCluster(clusters.GetMasterClusterName())
	if err != nil {
		nlog.Error(err)
		return err
	}
	if err := clusters.AddMasterProxyVirtualHost(c.Name, clusters.ServiceMasterProxy, domainID, base64.StdEncoding.EncodeToString(token)); err != nil {
		nlog.Error(err)
		return err
	}
	if err = xds.SetKeepAliveForDstCluster(c, true); err != nil {
		nlog.Error(err)
		return err
	}
	if err = xds.AddOrUpdateCluster(c); err != nil {
		nlog.Error(err)
		return err
	}
	return nil
}
