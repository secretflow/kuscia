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
	"reflect"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/handshake"
)

var (
	unitTestDeployToken = "FwBvarrLUpfACr00v8AiIbHbFcYguNqvu92XRJ2YysU="
	utAlice             = "alice"
	utBob               = "bob"
)

type RegisterJwtClaimsMd5 struct {
	ReqHash [16]byte `json:"req"` // deprecate soon
	jwt.RegisteredClaims
}

type RegisterJwtClaimsSha256 struct {
	ReqHashSha256 [32]byte `json:"req_hash"`
	jwt.RegisteredClaims
}

func generateJwtTokenMd5(namespace, csrData string, prikey *rsa.PrivateKey) (req *handshake.RegisterRequest, token string, err error) {
	req = &handshake.RegisterRequest{
		DomainId:    namespace,
		Csr:         base64.StdEncoding.EncodeToString([]byte(csrData)),
		RequestTime: int64(time.Now().Nanosecond()),
	}

	rjc := &RegisterJwtClaimsMd5{
		ReqHash: getRegisterRequestHashMd5(req),
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
	rawKey, err := base64.StdEncoding.DecodeString(keyStr)
	key, err = tls.ParseKey(rawKey, "")
	if err != nil {
		t.Errorf("Parse key data failed, error: %s.", err.Error())
	}
	csr = confloader.GenerateCsrData(namespace, keyStr, unitTestDeployToken)
	return
}

func TestRegisterDomain(t *testing.T) {
	csr, key := generateTestKey(t, utAlice)
	_ = RegisterDomain("alice", "test", csr, key, nil)
}

func TestVerifyRequest(t *testing.T) {
	csr, key := generateTestKey(t, utAlice)
	req, token, err := generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.NoError(t, err, "verifyRegisterRequest failed")
}

func TestVerifyCSRcn(t *testing.T) {
	// request domain is alice but csr is bob
	csr, key := generateTestKey(t, utBob)
	req, token, err := generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.Error(t, err, "verifyRegisterRequest failed")
}

func TestCompatibility(t *testing.T) {
	// token md5 vs claim (ma5\sha256)
	csr, key := generateTestKey(t, utAlice)
	req, token, err := generateJwtTokenMd5(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.NoError(t, err, "verifyRegisterRequest failed")

	// token (md5/sha256) vs claim (ma5)
	req, token, err = generateJwtToken(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequestMd5(req, token)
	assert.NoError(t, err, "generateJwtToken failed")

	// token (md5/sha256) vs claim (sha256)
	err = verifyRegisterRequestSha256(req, token)
	assert.NoError(t, err, "generateJwtToken failed")

	// token sha256 vs claim (ma5\sha256)
	req, token, err = generateJwtTokenSha256(utAlice, csr, key)
	assert.NoError(t, err, "generateJwtToken failed")
	err = verifyRegisterRequest(req, token)
	assert.NoError(t, err, "generateJwtToken failed")

}

func verifyRegisterRequestMd5(req *handshake.RegisterRequest, token string) error {
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
	if err = verifyJwtTokenMd5(token, certRequest.PublicKey, req); err != nil {
		return fmt.Errorf(`verify jwt failed, detail: %s`, err.Error())
	}
	return nil
}

func verifyJwtTokenMd5(jwtTokenStr string, pubKey interface{}, req *handshake.RegisterRequest) error {
	rjc := &RegisterJwtClaimsMd5{}
	jwtToken, err := jwt.ParseWithClaims(jwtTokenStr, rjc, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	})
	if err != nil {
		return err
	}
	if !jwtToken.Valid {
		return fmt.Errorf("%s", "jwt token decrpted fail")
	}
	if time.Since(rjc.ExpiresAt.Time) > 0 {
		return fmt.Errorf("%s", "verify jwt failed, detail: token expired")
	}
	// check md5 hash
	if reflect.DeepEqual(getRegisterRequestHashMd5(req), rjc.ReqHash) {
		return nil
	}
	return fmt.Errorf("verify request failed, detail: the request content doesn't match the hash")
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
		return fmt.Errorf("%s", "jwt token decrpted fail")
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
