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
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	RsaPKCS1PrivateKey = "RSA PRIVATE KEY"
	RsaPKCS8PrivateKey = "PRIVATE KEY"
	RsaPKCS1PublicKey  = "RSA PUBLIC KEY"
	RsaPKCS8PublicKey  = "PUBLIC KEY"
	CERTIFICATE        = "CERTIFICATE"
)

func ParseRSAPrivateKeyData(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid private key, should be PEM block format")
	}
	switch block.Type {
	case RsaPKCS1PrivateKey:
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case RsaPKCS8PrivateKey:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPriKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("invalid PKCS8 private key")
		}
		return rsaPriKey, nil
	default:
		return nil, fmt.Errorf("private key type [%s] not supported, must be RSA", block.Type)
	}
}

func ParseRSAPrivateKeyFile(serverKey string) (*rsa.PrivateKey, error) {
	priPemData, err := os.ReadFile(serverKey)
	if err != nil {
		return nil, err
	}
	return ParseRSAPrivateKeyData(priPemData)
}

func ParseRSAPublicKey(der []byte) (*rsa.PublicKey, error) {
	if len(der) == 0 {
		return nil, fmt.Errorf("public key is empty")
	}
	block, _ := pem.Decode(der)
	if block == nil {
		return nil, fmt.Errorf("public key should be PEM block format")
	}
	blockType := block.Type
	switch blockType {
	case RsaPKCS1PublicKey:
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case RsaPKCS8PublicKey:
		pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPubKey, ok := pubKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid PKCS8 public key")
		}
		return rsaPubKey, nil
	default:
		return nil, fmt.Errorf("public key type [%s] not supported, must be RSA", block.Type)
	}
}

func ParseCertData(data []byte) (*x509.Certificate, error) {
	certBlock, _ := pem.Decode(data)
	if certBlock == nil {
		return nil, fmt.Errorf("format error, must be cert")
	}
	return x509.ParseCertificate(certBlock.Bytes)
}

func ParseCertFromFile(caFilePath string) (*x509.Certificate, error) {
	certContent, err := os.ReadFile(caFilePath)
	if err != nil {
		return nil, err
	}

	return ParseCertData(certContent)
}

func EncodePKCS1PrivateKey(priKey *rsa.PrivateKey) []byte {
	block := &pem.Block{
		Type:  RsaPKCS1PrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(priKey),
	}
	return pem.EncodeToMemory(block)
}

func EncodePKCS8PrivateKey(priKey *rsa.PrivateKey) ([]byte, error) {
	data, err := x509.MarshalPKCS8PrivateKey(priKey)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  RsaPKCS8PrivateKey,
		Bytes: data,
	}
	return pem.EncodeToMemory(block), nil
}

func EncodePKCS1PublicKey(priKey *rsa.PrivateKey) []byte {
	block := &pem.Block{
		Type:  RsaPKCS1PublicKey,
		Bytes: x509.MarshalPKCS1PublicKey(&priKey.PublicKey),
	}
	return pem.EncodeToMemory(block)
}

func EncodePKCS8PublicKey(priKey *rsa.PrivateKey) ([]byte, error) {
	data, err := x509.MarshalPKIXPublicKey(&priKey.PublicKey)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  RsaPKCS8PublicKey,
		Bytes: data,
	}
	return pem.EncodeToMemory(block), nil
}

func EncryptPKCS1v15(pub *rsa.PublicKey, key []byte, prefix []byte) (string, error) {
	keyToEncrypt := append(prefix, key...)
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, keyToEncrypt)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
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
				return nil, fmt.Errorf("decrypt error, prefix not match")
			}
		}
		return key[len(prefix):], nil
	}

	return key[1:], nil
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

func EncryptOAEP(pub *rsa.PublicKey, key []byte) (string, error) {
	// refer: http://mpqs.free.fr/h11300-pkcs-1v2-2-rsa-cryptography-standard-wp_EMC_Corporation_Public-Key_Cryptography_Standards_(PKCS).pdf#page=18
	// see length check:
	//   2. If mLen > k - 2hLen - 2, output "message too long" and stop
	roundLength := pub.Size() - 2*sha256.New().Size() - 2
	start := 0
	ciphertext := make([]byte, 0)
	for start < len(key) {
		end := start + roundLength
		if end > len(key) {
			end = len(key)
		}
		currentCiphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, key[start:end], nil)
		if err != nil {
			return "", err
		}
		ciphertext = append(ciphertext, currentCiphertext...)
		start = end
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptOAEP(priv *rsa.PrivateKey, ciphertext string) ([]byte, error) {
	text, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	roundLength := priv.Size()
	start := 0
	plaintext := make([]byte, 0)
	for start < len(text) {
		end := start + roundLength
		if end > len(text) {
			end = len(text)
		}
		currentPlaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, text[start:end], nil)
		if err != nil {
			return nil, err
		}
		plaintext = append(plaintext, currentPlaintext...)
		start = end
	}
	return plaintext, nil
}

func VerifySSLKey(key []byte) bool {
	if _, err := ParseRSAPrivateKeyData(key); err != nil {
		return false
	}
	return true
}

func VerifyCert(cert []byte) bool {
	block, _ := pem.Decode(cert)
	if block == nil || block.Type != CERTIFICATE {
		return false
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return false
	}

	return true
}

func VerifyEncodeCert(base64EncodeCert string) error {
	data, err := base64.StdEncoding.DecodeString(base64EncodeCert)
	if err != nil {
		return fmt.Errorf("cert must be encoded with base64")
	}
	if !VerifyCert(data) {
		return fmt.Errorf("cert format is invalid, must be an x509 certificate")
	}
	return nil
}

func ParseEncodedKey(keyDataEncoded, keyFile string) (*rsa.PrivateKey, error) {
	var (
		keyDataDecoded []byte
		err            error
		key            *rsa.PrivateKey
	)
	if keyDataEncoded != "" {
		if keyDataDecoded, err = base64.StdEncoding.DecodeString(keyDataEncoded); err != nil {
			nlog.Errorf("Decoded keyData error: %v", err)
			return nil, err
		}
		key, err = ParseKey(keyDataDecoded, "")
		if err != nil {
			return nil, err
		}
		if keyFile != "" && !paths.CheckFileExist(keyFile) {
			if err = paths.WriteFile(keyFile, keyDataDecoded); err != nil {
				return nil, err
			}
		}
		return key, nil
	}
	return ParseKey(nil, keyFile)
}

func ParseKey(keyData []byte, keyFile string) (key *rsa.PrivateKey, err error) {
	if len(keyData) == 0 && keyFile == "" {
		return nil, fmt.Errorf("init key failed: should not all empty")
	}
	// Make key: keyData's priority is higher than keyFile
	if len(keyData) != 0 {
		key, err = ParseRSAPrivateKeyData(keyData)
		if err == nil {
			return key, nil
		}
		nlog.Errorf("load key data failed: %s, try key file", err)
	}
	if key == nil && keyFile != "" {
		key, err = ParseRSAPrivateKeyFile(keyFile)
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
		cert, err = ParseCertData(certData)
		if err == nil {
			return cert, nil
		}
		nlog.Errorf("load cert data failed: %s, try cert file", err)
	}
	if cert == nil && certFile != "" {
		cert, err = ParseCertFromFile(certFile)
		if err == nil {
			return cert, nil
		}
		nlog.Errorf("load cert file failed: %s", err)
	}

	return nil, fmt.Errorf("can't parse cert")
}

func ParseCertWithGenerated(privateKey *rsa.PrivateKey, subject string, certData []byte, certFile string) (cert *x509.Certificate, err error) {
	if len(certData) != 0 || (certFile != "" && paths.CheckFileExist(certFile)) {
		return ParseCert(certData, certFile)
	}

	nlog.Infof("Generate cert with key, subject[%s]", subject)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: subject},
		PublicKeyAlgorithm:    x509.RSA,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(50, 0, 0),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	crtRaw, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		nlog.Errorf("Create certificate error: %v", err)
		return nil, err
	}

	certOut, err := os.Create(certFile)
	defer certOut.Close()
	if err != nil {
		nlog.Errorf("Create cert file [%s] error: %v", certFile, err)
		return nil, err
	}
	err = pem.Encode(certOut, &pem.Block{
		Type:  CERTIFICATE,
		Bytes: crtRaw,
	})
	if err != nil {
		nlog.Errorf("Encode cert error: %v", err)
		return nil, err
	}

	return x509.ParseCertificate(crtRaw)
}

func GenerateKeyCertPairData(rootCAKey *rsa.PrivateKey, rootCACert *x509.Certificate, commonName string) (string, string, error) {
	var certBuf, keyBuf bytes.Buffer

	var netIPs []net.IP
	netIPs = append(netIPs, net.IPv4(127, 0, 0, 1))

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{CommonName: commonName},
		IPAddresses:  netIPs,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	err := GenerateX509KeyPair(rootCACert, rootCAKey, cert, &certBuf, &keyBuf)
	if err != nil {
		return "", "", err
	}
	return keyBuf.String(), certBuf.String(), nil
}

func LoadKeyData(keyFile string) (string, error) {
	priPemData, err := os.ReadFile(keyFile)
	if err != nil {
		return "", err
	}
	if _, err = ParseRSAPrivateKeyData(priPemData); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(priPemData), nil
}

func GenerateKeyData() (string, error) {
	var keyOut bytes.Buffer
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	err = pem.Encode(&keyOut, &pem.Block{
		Type:  RsaPKCS1PrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return base64.StdEncoding.EncodeToString(keyOut.Bytes()), err
}

func WritePrivateKeyToFile(key *rsa.PrivateKey, filename string) error {
	keyOut, err := os.Create(filename)
	defer keyOut.Close()
	if err != nil {
		return fmt.Errorf("create key file [%s] error: %v", filename, err.Error())
	}
	return pem.Encode(keyOut, &pem.Block{
		Type:  RsaPKCS1PrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func WriteX509CertToFile(cert *x509.Certificate, filename string) error {
	certOut, err := os.Create(filename)
	defer certOut.Close()
	if err != nil {
		return fmt.Errorf("create key file [%s] error: %v", filename, err.Error())
	}
	return pem.Encode(certOut, &pem.Block{
		Type:  CERTIFICATE,
		Bytes: cert.Raw,
	})
}

func GeneratePrivateKeyToFile(filename string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	return WritePrivateKeyToFile(priv, filename)
}

func SignWithRSA(key *rsa.PrivateKey, data string) (string, error) {
	h := sha256.New()
	h.Write([]byte(data))
	digest := h.Sum(nil)
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sigBytes), nil
}
