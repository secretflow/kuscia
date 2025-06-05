// Copyright 2024 Ant Group Co., Ltd.
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
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	gomonkeyv2 "github.com/agiledragon/gomonkey/v2"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/handshake"
)

var (
	unitTestDeployToken = "FwBvarrLUpfACr00v8AiIbHbFcYguNqvu92XRJ2YysU="
	utAlice             = "alice"
	utBob               = "bob"
)

type RegisterJwtClaimsSha256 struct {
	ReqHashSha256 [32]byte `json:"req_hash"`
	jwt.RegisteredClaims
}

func generateJwtTokenSha256(namespace, csrData string, prikey *rsa.PrivateKey) (req *handshake.RegisterRequest, token string, err error) {
	req = &handshake.RegisterRequest{
		DomainId:    namespace,
		Csr:         base64.StdEncoding.EncodeToString([]byte(csrData)),
		RequestTime: int64(time.Now().Nanosecond()),
	}

	rjc := &RegisterJwtClaimsSha256{
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

func generateTestKey(t *testing.T, namespace string) (csr string, key *rsa.PrivateKey) {
	keyStr, err := tls.GenerateKeyData()
	if err != nil {
		t.Errorf("Generate key data failed, error: %s.", err.Error())
	}
	rawKey, _ := base64.StdEncoding.DecodeString(keyStr)
	key, err = tls.ParseKey(rawKey, "")
	if err != nil {
		t.Errorf("Parse key data failed, error: %s.", err.Error())
	}
	csr = confloader.GenerateCsrData(namespace, keyStr, unitTestDeployToken)
	return
}

func TestRegisterDomain_ServerNotExists(t *testing.T) {
	t.Parallel()
	csr, key := generateTestKey(t, utAlice)

	// try to mock http request
	gomonkeyv2.ApplyFunc(utils.DoHTTPWithRetry, func(i interface{}, out interface{}, hp *utils.HTTPParam, d time.Duration, tm int) error {
		assert.Equal(t, http.MethodPost, hp.Method)
		assert.Equal(t, utAlice, hp.KusciaSource)
		return nil
	})

	assert.NoError(t, RegisterDomain("alice", "test", csr, key, nil))
}

func TestVerifyRequest(t *testing.T) {
	t.Parallel()
	csr, key := generateTestKey(t, utAlice)
	req, token, err := generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.NoError(t, err, "verifyRegisterRequest failed")
}

func TestVerifyCSRcn(t *testing.T) {
	t.Parallel()
	// request domain is alice but csr is bob
	csr, key := generateTestKey(t, utBob)
	req, token, err := generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.Error(t, err, "verifyRegisterRequest failed")
}

func TestCompatibility(t *testing.T) {
	t.Parallel()
	csr, key := generateTestKey(t, utAlice)

	req, token, err := generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")

	err = verifyRegisterRequestSha256(req, token)
	assert.NoError(t, err, "generateJwtToken failed")

	req, token, err = generateJwtTokenSha256(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.NoError(t, err, "generateJwtToken failed")

}

func verifyRegisterRequestSha256(req *handshake.RegisterRequest, token string) error {
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
	if err = verifyJwtTokenSha256(token, certRequest.PublicKey, req); err != nil {
		return fmt.Errorf("verify jwt failed, detail: %s", err.Error())
	}
	return nil
}

func verifyJwtTokenSha256(jwtTokenStr string, pubKey interface{}, req *handshake.RegisterRequest) error {
	rjc := &RegisterJwtClaimsSha256{}
	jwtToken, err := jwt.ParseWithClaims(jwtTokenStr, rjc, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		return err
	}
	if !jwtToken.Valid {
		return fmt.Errorf("%s", "jwt token decrypted fail")
	}
	if time.Since(rjc.ExpiresAt.Time) > 0 {
		return fmt.Errorf("%s", "jwt verify error, token expired")
	}
	// check sha256 hash
	if reflect.DeepEqual(getRegisterRequestHashSha256(req), rjc.ReqHashSha256) {
		return nil
	}
	return fmt.Errorf("verify request failed, detail: the request content doesn't match the hash")
}
