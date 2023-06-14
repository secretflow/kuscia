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

package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func ParsePKCS1PrivateKeyData(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid private key, should be PEM block format")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func ParsePKCS1PrivateKey(serverKey string) (*rsa.PrivateKey, error) {
	priPemData, err := os.ReadFile(serverKey)
	if err != nil {
		return nil, err
	}

	return ParsePKCS1PrivateKeyData(priPemData)
}

func ParsePKCS1PublicKey(der []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(der)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		return nil, fmt.Errorf("invalid public key, should be PEM block format")
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func ParsePKCS1CertFromFile(caFilePath string) (*x509.Certificate, error) {
	certContent, err := os.ReadFile(caFilePath)
	if err != nil {
		return nil, err
	}
	certBlock, _ := pem.Decode(certContent)
	if certBlock == nil {
		return nil, fmt.Errorf("%s format error, must be cert", caFilePath)
	}
	return x509.ParseCertificate(certBlock.Bytes)
}

func EncodePKCS1PublicKey(priKey *rsa.PrivateKey) []byte {
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&priKey.PublicKey),
	}
	pubPem := pem.EncodeToMemory(block)
	return pubPem
}

func EncryptPKCS1v15(pub *rsa.PublicKey, key []byte) (string, error) {
	rng := rand.Reader
	ciphertext, err := rsa.EncryptPKCS1v15(rng, pub, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptPKCS1v15(priv *rsa.PrivateKey, ciphertext string, keysize int) ([]byte, error) {
	rng := rand.Reader

	key := make([]byte, keysize)
	if _, err := io.ReadFull(rng, key); err != nil {
		return nil, err
	}

	text, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	if err := rsa.DecryptPKCS1v15SessionKey(rng, priv, text, key); err != nil {
		return nil, err
	}

	return key, nil
}

func VerifySSLKey(key []byte) bool {
	block, _ := pem.Decode(key)
	if block == nil {
		return false
	}

	if block.Type == "RSA PRIVATE KEY" {
		if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			return false
		}
		return true
	}

	if block.Type == "EC PRIVATE KEY" {
		if _, err := x509.ParseECPrivateKey(block.Bytes); err != nil {
			return false
		}
		return true
	}

	nlog.Errorf("invalid block type:%v", block.Type)
	return false
}

func VerifyCert(cert []byte) bool {
	block, _ := pem.Decode(cert)
	if block == nil || block.Type != "CERTIFICATE" {
		return false
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return false
	}

	return true
}
