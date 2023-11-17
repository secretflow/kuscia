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

package tls

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

func ParsePKCS1CertData(data []byte) (*x509.Certificate, error) {
	certBlock, _ := pem.Decode(data)
	if certBlock == nil {
		return nil, fmt.Errorf("format error, must be cert")
	}
	return x509.ParseCertificate(certBlock.Bytes)
}

func ParsePKCS1CertFromFile(caFilePath string) (*x509.Certificate, error) {
	certContent, err := os.ReadFile(caFilePath)
	if err != nil {
		return nil, err
	}

	return ParsePKCS1CertData(certContent)
}

func EncodePKCS1PublicKey(priKey *rsa.PrivateKey) []byte {
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&priKey.PublicKey),
	}
	pubPem := pem.EncodeToMemory(block)
	return pubPem
}

func EncryptPKCS1v15(pub *rsa.PublicKey, key []byte, prefix []byte) (string, error) {
	keyToEncrypt := append(prefix, key...)
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, keyToEncrypt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func generateRandKey(keysize int) ([]byte, error) {
	key := make([]byte, keysize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func genKey(keysize int, prefix []byte) ([]byte, error) {
	key, err := generateRandKey(keysize + len(prefix))
	if err != nil {
		return nil, err
	}

	if len(prefix) > 0 {
		i := 0
		for i < len(prefix) {
			if key[i] != prefix[i] {
				return key, nil
			}
			i++
		}
		return genKey(keysize, prefix)
	}
	return key, nil
}

func DecryptPKCS1v15(priv *rsa.PrivateKey, ciphertext string, keysize int, prefix []byte) ([]byte, error) {
	key, err := genKey(keysize, prefix)
	if err != nil {
		return nil, err
	}

	text, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	if err := rsa.DecryptPKCS1v15SessionKey(rand.Reader, priv, text, key); err != nil {
		return nil, err
	}

	if len(prefix) > 0 {
		i := 0
		for ; i < len(prefix); i++ {
			if key[i] != prefix[i] {
				return nil, fmt.Errorf("decrypt error")
			}
		}
		return key[len(prefix):], nil
	}

	return key[1:], nil
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

func ParseKey(keyData []byte, keyFile string) (key *rsa.PrivateKey, err error) {
	if len(keyData) == 0 && keyFile == "" {
		return nil, fmt.Errorf("init key failed: should not all empty")
	}
	// Make key: keyData's priority is higher than keyFile
	if len(keyData) != 0 {
		key, err = ParsePKCS1PrivateKeyData(keyData)
		if err == nil {
			return key, nil
		}
		nlog.Errorf("load key data failed: %s, try key file", err)
	}
	if key == nil && keyFile != "" {
		key, err = ParsePKCS1PrivateKey(keyFile)
		if err == nil {
			return key, nil
		}
		nlog.Errorf("load key file failed: %s", err)
	}
	return nil, fmt.Errorf("can't parse key")
}

func ParseCert(certData []byte, certFile string) (cert *x509.Certificate, err error) {
	if len(certData) == 0 && certFile == "" {
		return nil, fmt.Errorf("init cert failed: should not all empty")
	}
	// Make cert:  certData's priority is higher than certFile
	if len(certData) != 0 {
		cert, err = ParsePKCS1CertData(certData)
		if err == nil {
			return cert, nil
		}
		nlog.Errorf("load cert data failed: %s, try cert file", err)
	}
	if cert == nil && certFile != "" {
		cert, err = ParsePKCS1CertFromFile(certFile)
		if err == nil {
			return cert, nil
		}
		nlog.Errorf("load cert file failed: %s", err)
	}

	return nil, fmt.Errorf("can't parse cert")
}
