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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	tokenByteSize = 32
	NoopToken     = "noop"
)

const (
	syncUpdate  = "DomainRouteSyncUpdate"
	syncDelete  = "DomainRouteSyncDelete"
	syncSucceed = "DomainRouteSyncSucceed"
	syncFailed  = "DomainRouteSyncFailed"
)

type Token struct {
	Token   string
	Version int64
}

func (c *DomainRouteController) startHandShakeServer(port uint32) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handShakeHandle)
	c.handshakeServer = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	nlog.Error(c.handshakeServer.ListenAndServe())
}

func (c *DomainRouteController) sourceInitiateHandShake(dr *kusciaapisv1alpha1.DomainRoute) error {
	if dr.Spec.TokenConfig.SourcePublicKey != c.gateway.Status.PublicKey {
		nlog.Errorf("DomainRoute %s: mismatch source public key", dr.Name)
		return nil
	}
	destPub, err := base64.StdEncoding.DecodeString(dr.Spec.TokenConfig.DestinationPublicKey)
	if err != nil {
		nlog.Errorf("DomainRoute %s: destination public key format error, must be base64 encoded", dr.Name)
		return err
	}
	destPubKey, err := utils.ParsePKCS1PublicKey(destPub)
	if err != nil {
		return err
	}

	sourceToken := make([]byte, tokenByteSize/2)
	_, err = rand.Read(sourceToken)
	if err != nil {
		return err
	}

	drs := kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dr.Name,
			Namespace: dr.Namespace,
		},
		Spec:   dr.Spec,
		Status: dr.Status,
	}
	drs.Status.TokenStatus.Tokens = nil
	drs.Status.TokenStatus.RevisionToken.Token, err = encryptToken(destPubKey, sourceToken)
	if err != nil {
		return err
	}

	maxRetryTimes := 5
	var statusCode int
	var body []byte
	for i := 0; i < maxRetryTimes; i++ {
		body, _ = json.Marshal(drs)
		req, err := http.NewRequest("POST", InternalServer, bytes.NewBuffer(body))
		if err != nil {
			nlog.Warnf("new handshake request fail:%v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		ns := dr.Spec.Destination
		if dr.Spec.Transit != nil {
			ns = dr.Spec.Transit.Domain.DomainID
		}
		handshakeCluster := fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, ns, dr.Spec.Endpoint.Ports[0].Name)
		req.Header.Set("Kuscia-Handshake-Cluster", handshakeCluster)
		req.Header.Set("kuscia-Host", fmt.Sprintf("kuscia-handshake.%s.svc", ns))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			nlog.Errorf("do http request fail:%v", err)
			if i == maxRetryTimes-1 {
				return err
			}
			time.Sleep(time.Second)
			continue
		}

		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			nlog.Errorf("handshake with %s fail: %v", handshakeCluster, err)
			return err
		}

		statusCode = resp.StatusCode
		if statusCode != http.StatusOK {
			nlog.Errorf("DomainRoute %s/%s: %d, error: %s", dr.Namespace, dr.Name, resp.StatusCode, string(body))
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("handshake status code: %d", statusCode)
	}

	var drd kusciaapisv1alpha1.DomainRoute
	if err = json.Unmarshal(body, &drd); err != nil {
		return err
	}

	destToken, err := decryptToken(c.prikey, drd.Status.TokenStatus.RevisionToken.Token, tokenByteSize/2)
	if err != nil {
		return err
	}

	token := append(sourceToken, destToken...)
	tokenEncrypted, err := encryptToken(&c.prikey.PublicKey, token)
	if err != nil {
		return err
	}

	dr = dr.DeepCopy()
	dr.Status.TokenStatus.RevisionToken.Token = tokenEncrypted
	_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr, metav1.UpdateOptions{})
	return err
}

func (c *DomainRouteController) handShakeHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(fmt.Sprintf("{\"namespace\":\"%s\"}", c.gateway.Namespace)))
		if err != nil {
			nlog.Errorf("write handshake response fail, detail-> %v", err)
		}
		return
	}

	var drs kusciaapisv1alpha1.DomainRoute
	err := json.NewDecoder(r.Body).Decode(&drs)
	if err != nil {
		nlog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	drd, err := c.domainRouteLister.DomainRoutes(c.gateway.Namespace).Get(drs.Name)
	if err != nil {
		msg := fmt.Sprintf("DomainRoute %s get error: %v", drs.Name, err)
		nlog.Error(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	dr, err := c.DestReplyHandshake(&drs, drd)
	if err != nil && !k8serrors.IsNotFound(err) {
		msg := fmt.Sprintf("DomainRoute %s handle error: %v", drs.Name, err)
		nlog.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(dr)
	if err != nil {
		nlog.Errorf("encode handshake response for(%s) fail, detail-> %v", drs.Name, err)
	}
	nlog.Infof("DomainRoute %s handle success", drs.Name)
}

func (c *DomainRouteController) DestReplyHandshake(drs, drd *kusciaapisv1alpha1.DomainRoute) (*kusciaapisv1alpha1.DomainRoute, error) {
	srcAuth, dstAuth := drs.Spec.TokenConfig, drd.Spec.TokenConfig
	if srcAuth.DestinationPublicKey != c.gateway.Status.PublicKey {
		return nil, fmt.Errorf("[token-handshake] mismatch incoming destination rsa public key")
	}

	if dstAuth.SourcePublicKey != srcAuth.SourcePublicKey || dstAuth.DestinationPublicKey != srcAuth.DestinationPublicKey {
		return nil, fmt.Errorf("[token-handshake] mismatch source or destination rsa public key")
	}

	srcRevisionToken, dstRevisionToken := drs.Status.TokenStatus.RevisionToken, drd.Status.TokenStatus.RevisionToken
	if dstRevisionToken.Revision != srcRevisionToken.Revision {
		return nil, fmt.Errorf("[token-handshake] mismatch revision, source: %d, destination: %d",
			srcRevisionToken.Revision, dstRevisionToken.Revision)
	}
	srcPub, err := base64.StdEncoding.DecodeString(srcAuth.SourcePublicKey)
	if err != nil {
		return nil, err
	}
	sourcePubKey, err := utils.ParsePKCS1PublicKey(srcPub)
	if err != nil {
		return nil, err
	}

	sourceToken, err := decryptToken(c.prikey, srcRevisionToken.Token, tokenByteSize/2)
	if err != nil {
		return nil, err
	}

	destToken := make([]byte, tokenByteSize/2)
	if _, err = rand.Read(destToken); err != nil {
		return nil, err
	}

	token := append(sourceToken, destToken...)
	tokenEncrypted, err := encryptToken(&c.prikey.PublicKey, token)
	if err != nil {
		return nil, err
	}

	drd = drd.DeepCopy()
	drd.Status.TokenStatus.RevisionToken.Token = tokenEncrypted
	if drd, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(drd.Namespace).UpdateStatus(context.Background(), drd,
		metav1.UpdateOptions{}); err != nil {
		return nil, err
	}

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drd.Name,
			Namespace: drd.Namespace,
		},
		Spec:   drd.Spec,
		Status: drd.Status,
	}
	dr.Status.TokenStatus.Tokens = nil
	dr.Status.TokenStatus.RevisionToken.Token, err = encryptToken(sourcePubKey, destToken)
	if err != nil {
		return nil, err
	}

	return dr, nil
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
	case kusciaapisv1alpha1.TokenGenMethodRAND:
		tokens, err = c.parseTokenRand(dr)
	case kusciaapisv1alpha1.TokenGenMethodRSA:
		tokens, err = c.parseTokenRSA(dr)
	default:
		err = fmt.Errorf("DomainRoute %s unsupported token method: %s", routeKey,
			dr.Spec.TokenConfig.TokenGenMethod)
	}
	return tokens, err
}

func (c *DomainRouteController) parseTokenRand(dr *kusciaapisv1alpha1.DomainRoute) ([]*Token, error) {
	key, _ := cache.MetaNamespaceKeyFunc(dr)

	var tokens []*Token
	n := len(dr.Status.TokenStatus.Tokens)
	if n == 0 {
		return tokens, fmt.Errorf("DomainRoute %s has no avaliable token", key)
	}

	for _, token := range dr.Status.TokenStatus.Tokens {
		tokens = append(tokens, &Token{Token: token.Token, Version: token.Revision})
	}

	return tokens, nil
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
		c.recorder.Event(c.gateway, corev1.EventTypeWarning, syncFailed, err.Error())
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
			dr.Status.TokenStatus.Tokens[i].EffectiveInstances = append(dr.Status.TokenStatus.Tokens[i].
				EffectiveInstances, c.gateway.Name)
		}
	}

	if updated {
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(context.Background(), dr,
			metav1.UpdateOptions{}); err != nil {
			nlog.Error(err)
			return err
		}
	}
	return nil
}

func encryptToken(pub *rsa.PublicKey, key []byte) (string, error) {
	return utils.EncryptPKCS1v15(pub, key)
}

func decryptToken(priv *rsa.PrivateKey, ciphertext string, keysize int) ([]byte, error) {
	return utils.DecryptPKCS1v15(priv, ciphertext, keysize)
}

func exists(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
