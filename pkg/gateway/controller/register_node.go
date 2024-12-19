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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/handshake"
)

func getRegisterRequestHashSha256(regReq *handshake.RegisterRequest) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("%s_%s_%d", regReq.DomainId, regReq.Csr, regReq.RequestTime)))
}

type RegisterJwtClaims struct {
	ReqHashSha256 [32]byte `json:"req_hash"`
	jwt.RegisteredClaims
}

func RegisterDomain(namespace, path, csrData string, priKey *rsa.PrivateKey, afterRegisterHook AfterRegisterDomainHook) error {
	req, token, err := generateJwtToken(namespace, csrData, priKey)
	if err != nil {
		return err
	}
	regResp := &handshake.RegisterResponse{}
	headers := map[string]string{
		"jwt-token": token,
	}
	err = doHTTPWithDefaultRetry(req, regResp, &utils.HTTPParam{
		Method:       http.MethodPost,
		Path:         fmt.Sprintf("%s/register", strings.TrimSuffix(path, "/")),
		KusciaSource: namespace,
		ClusterName:  clusters.GetMasterClusterName(),
		KusciaHost:   fmt.Sprintf("%s.master.svc", utils.ServiceHandshake),
		Headers:      headers,
	})
	if err != nil {
		return err
	}
	if afterRegisterHook != nil {
		afterRegisterHook(regResp)
	}
	return nil
}

func generateJwtToken(namespace, csrData string, prikey *rsa.PrivateKey) (req *handshake.RegisterRequest, token string, err error) {
	req = &handshake.RegisterRequest{
		DomainId:    namespace,
		Csr:         base64.StdEncoding.EncodeToString([]byte(csrData)),
		RequestTime: int64(time.Now().Nanosecond()),
	}

	rjc := &RegisterJwtClaims{
		ReqHashSha256: getRegisterRequestHashSha256(req),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    namespace,
			Subject:   namespace,
		},
	}
	tokenData := jwt.NewWithClaims(jwt.SigningMethodRS256, rjc)
	token, err = tokenData.SignedString(prikey)
	if err != nil {
		nlog.Errorf("Signed token failed, error: %s.", err.Error())
	}
	return
}

func (c *DomainRouteController) registerHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req := handshake.RegisterRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		httpErrWrapped(w, err, http.StatusBadRequest)
		return
	}

	// Csr in request must be base64 encoded string
	// Raw data must be pem format
	certRequest, err := parseCertRequest(req.Csr)
	if err != nil {
		httpErrWrapped(w, err, http.StatusBadRequest)
		return
	}
	// verify jwt token and csr
	err = verifyRegisterRequest(&req, r.Header.Get("jwt-token"))
	if err != nil {
		httpErrWrapped(w, err, http.StatusBadRequest)
		return
	}

	// create domain certificate
	t := time.Unix(req.RequestTime/int64(time.Second), req.RequestTime%int64(time.Second))
	domainCrt := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               certRequest.Subject,
		PublicKeyAlgorithm:    certRequest.PublicKeyAlgorithm,
		PublicKey:             certRequest.PublicKey,
		NotBefore:             t,
		NotAfter:              t.AddDate(10, 0, 0),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	domainCrtRaw, err := x509.CreateCertificate(rand.Reader, domainCrt, c.CaCert, certRequest.PublicKey, c.CaKey)
	if err != nil {
		httpErrWrapped(w, err, http.StatusInternalServerError)
		return
	}
	domainCrtStr := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: domainCrtRaw}))

	domain, err := c.kusciaClient.KusciaV1alpha1().Domains().Get(context.Background(), req.DomainId, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			httpErrWrapped(w, fmt.Errorf("domain [%s] not found", req.DomainId), http.StatusInternalServerError)
		} else {
			httpErrWrapped(w, fmt.Errorf("get domain [%s] info failed, detail -> %s", req.DomainId, err.Error()), http.StatusInternalServerError)
		}
		return
	}
	if domain.Status == nil {
		httpErrWrapped(w, fmt.Errorf("deploytokenstatus of domain is empty"), http.StatusInternalServerError)
		return
	}

	// If the tokens match and the cert in the domain does not match the cert in the request, the domain is updated
	if !isCertMatch(domain.Spec.Cert, domainCrt) {
		index, err := deployTokenMatched(certRequest, domain.Status.DeployTokenStatuses)
		if err != nil {
			httpErrWrapped(w, fmt.Errorf("source domain [%s] deploy token mismatch, detail -> %s", req.DomainId, err.Error()), http.StatusInternalServerError)
			return
		}

		domainDeepCopy := domain.DeepCopy()
		domainDeepCopy.Spec.Cert = domainCrtStr
		domainDeepCopy.Status.DeployTokenStatuses[index].State = common.DeployTokenUsedState
		// update domain status
		if _, err = c.kusciaClient.KusciaV1alpha1().Domains().UpdateStatus(context.Background(), domainDeepCopy, metav1.UpdateOptions{}); err != nil {
			if k8serrors.IsNotFound(err) {
				httpErrWrapped(w, fmt.Errorf("source domain [%s] not found, may be deleted", req.DomainId), http.StatusInternalServerError)
			} else {
				httpErrWrapped(w, fmt.Errorf("get source domain [%s] error, detail -> %s", req.DomainId, err.Error()), http.StatusInternalServerError)
			}
			return
		}
		// update domain cert
		oldData, _ := json.Marshal(kusciaapisv1alpha1.Domain{Spec: domain.Spec})
		newData, _ := json.Marshal(kusciaapisv1alpha1.Domain{Spec: domainDeepCopy.Spec})
		patchBytes, _ := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.Domain{})
		if _, err = c.kusciaClient.KusciaV1alpha1().Domains().Patch(context.Background(), domainDeepCopy.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			if k8serrors.IsNotFound(err) {
				httpErrWrapped(w, fmt.Errorf("source domain [%s] not found, may be deleted", req.DomainId), http.StatusInternalServerError)
			} else {
				httpErrWrapped(w, fmt.Errorf("get source domain [%s] error, detail -> %s", req.DomainId, err.Error()), http.StatusInternalServerError)
			}
			return
		}
		nlog.Infof("Domain %s register success, set domain cert", domain.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&handshake.RegisterResponse{
		Cert: domainCrtStr,
	})
	if err != nil {
		nlog.Errorf("encode register response for(%s) fail, detail-> %v", req.DomainId, err)
	} else {
		nlog.Infof("Domain register %s handle success", req.DomainId)
	}
}

func deployTokenMatched(certRequest *x509.CertificateRequest, deployTokenStatuses []kusciaapisv1alpha1.DeployTokenStatus) (int, error) {
	regToken := getTokenFromCertRequest(certRequest)
	if regToken == "" {
		return -1, fmt.Errorf("%s", "token not found in cert request, bad request")
	}
	// Check whether the token in the certRequest exists
	for i, dts := range deployTokenStatuses {
		if dts.Token == regToken {
			if dts.State == common.DeployTokenUnusedState {
				return i, nil
			}
			return i, fmt.Errorf("deploy token found but used, please use an unused deploy token")
		}
	}
	return -1, fmt.Errorf("deploy token not found, please use a valid unused deploy token")
}

func httpErrWrapped(w http.ResponseWriter, err error, statusCode int) {
	nlog.Error(err)
	http.Error(w, err.Error(), statusCode)
}

func isCertMatch(certString string, c *x509.Certificate) bool {
	if certString == "" {
		return false
	}
	certPem, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return false
	}
	certData, _ := pem.Decode(certPem)
	if certData == nil {
		return false
	}
	cert, err := x509.ParseCertificate(certData.Bytes)
	if err != nil {
		return false
	}
	if !reflect.DeepEqual(cert.PublicKey, c.PublicKey) {
		nlog.Error("public not match")
		return false
	}
	if !reflect.DeepEqual(cert.Subject, c.Subject) {
		nlog.Error("subject not match")
		return false
	}
	if !reflect.DeepEqual(cert.PublicKeyAlgorithm, c.PublicKeyAlgorithm) {
		nlog.Error("PublicKeyAlgorithm not match")
		return false
	}
	return true
}

func verifyJwtToken(jwtTokenStr string, pubKey interface{}, req *handshake.RegisterRequest) error {
	rjc := &RegisterJwtClaims{}
	jwtToken, err := jwt.ParseWithClaims(jwtTokenStr, rjc, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		return err
	}
	if !jwtToken.Valid {
		return fmt.Errorf("%s", "verify jwt failed, detail: jwt token decrpted fail")
	}
	if time.Since(rjc.ExpiresAt.Time) > 0 {
		return fmt.Errorf("%s", "verify jwt failed, detail: token expired")
	}
	// check sha256 hash
	if reflect.DeepEqual(getRegisterRequestHashSha256(req), rjc.ReqHashSha256) {
		return nil
	}
	return fmt.Errorf("verify jwt failed, detail: the request content doesn't match the hash")
}

// verifyCSR verify the CN of CSR must be equal with domainID
func verifyCSR(csr *x509.CertificateRequest, domainID string) error {

	if csr == nil {
		return fmt.Errorf("csr is nil")
	}
	if csr.Subject.CommonName != domainID {
		return fmt.Errorf("the csr subject common name must be domainID: %s not %s", csr.Subject.CommonName, domainID)
	}
	return nil
}

func verifyRegisterRequest(req *handshake.RegisterRequest, token string) error {
	// Csr in request must be base64 encoded string
	// Raw data must be pem format
	certRequest, err := parseCertRequest(req.Csr)
	if err != nil {
		return fmt.Errorf("parse cert request failed, detail: %s", err.Error())
	}
	// verify the CN of CSR must be equal with domainID
	if err = verifyCSR(certRequest, req.DomainId); err != nil {
		return fmt.Errorf("verify csr failed, detail: %s", err.Error())
	}
	// Use jwt verify first.
	// JWT token must be signed by domain's private key.
	// This handler will verify it by public key in csr.
	if err = verifyJwtToken(token, certRequest.PublicKey, req); err != nil {
		return fmt.Errorf("verify jwt failed, detail: %s", err.Error())
	}
	return nil
}

// The token in the csr file must be in the extension field and its id must be 1.2.3.4
func getTokenFromCertRequest(certRequest *x509.CertificateRequest) string {
	for _, e := range certRequest.Extensions {
		if e.Id.String() == common.DomainCsrExtensionID {
			return string(e.Value)
		}
	}
	return ""
}

func parseCertRequest(certStr string) (*x509.CertificateRequest, error) {
	csrRawPem, err := base64.StdEncoding.DecodeString(certStr)
	if err != nil {
		err = fmt.Errorf("base64 decode csr error: %s", err.Error())
		return nil, err
	}
	p, _ := pem.Decode(csrRawPem)
	if p == nil {
		err = fmt.Errorf("%s", "pem decode csr error")
		return nil, err
	}
	if p.Type != "CERTIFICATE REQUEST" {
		err = fmt.Errorf(`csr pem data type is %s, must be "CERTIFICATE REQUEST"`, p.Type)
		return nil, err
	}
	csr, err := x509.ParseCertificateRequest(p.Bytes)
	if err != nil {
		err = fmt.Errorf("csr pem data parse error: %s", err.Error())
		return nil, err
	}
	return csr, nil
}
