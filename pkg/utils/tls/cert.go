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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
)

// LoadX509KeyPair reads and parses a public/private key pair from a pair
// of files. The files must contain PEM encoded data.
func LoadX509KeyPair(certFile, keyFile string) (*x509.Certificate, *rsa.PrivateKey, error) {
	cert, keyBlock, err := loadKeyPair(certFile, keyFile)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key, detail-> %v", err)
	}

	return cert, key, nil
}

func LoadX509EcKeyPair(certFile, keyFile string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cert, keyBlock, err := loadKeyPair(certFile, keyFile)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key, detail-> %v", err)
	}

	return cert, key, nil
}

func loadKeyPair(certFile, keyFile string) (*x509.Certificate, *pem.Block, error) {
	certContent, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate file %q, detail-> %v", certFile, err)
	}

	certBlock, _ := pem.Decode(certContent)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate, detail-> %v", err)
	}

	keyContent, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read key file %q, detail-> %v", keyFile, err)
	}

	keyBlock, _ := pem.Decode(keyContent)
	return cert, keyBlock, nil
}

// GenerateX509KeyPair creates a public/private key pair and creates a new X.509 v3 certificate based on a template.
// caKey can be ecdsa.PrivateKey or rsa.PrivateKey
func GenerateX509KeyPair(parent *x509.Certificate, caKey any, cert *x509.Certificate, certOut, keyOut io.Writer) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate rsa key, detail-> %v", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, parent, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create x509 certificate, detail-> %v", err)
	}

	if err = pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return fmt.Errorf("failed to encode cert, detail-> %v", err)
	}

	if err = pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		return fmt.Errorf("failed to encode key, detail-> %v", err)
	}

	return nil
}

// GenerateX509KeyPairStruct creates a public/private key pair and creates a new X.509 v3 certificate based on a template.
// caKey can be ecdsa.PrivateKey or rsa.PrivateKey
func GenerateX509KeyPairStruct(parent *x509.Certificate, caKey any, certTemplate *x509.Certificate) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate rsa key, detail-> %v", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, parent, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create x509 certificate, detail-> %v", err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate, detail-> %v", err)
	}

	return key, cert, nil
}

// EncodeRsaKeyToPKCS1 encode key to pkcs#1 form key.
func EncodeRsaKeyToPKCS1(key *rsa.PrivateKey) (string, error) {
	var keyOut bytes.Buffer
	if err := pem.Encode(&keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		return "", fmt.Errorf("failed to encode key, detail-> %v", err)
	}

	return keyOut.String(), nil
}

// EncodeRsaKeyToPKCS8 encode key to pkcs#8 form key.
func EncodeRsaKeyToPKCS8(key *rsa.PrivateKey) (string, error) {
	var keyOut bytes.Buffer
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	if err := pem.Encode(&keyOut, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}); err != nil {
		return "", fmt.Errorf("failed to encode key, detail-> %v", err)
	}

	return keyOut.String(), nil
}

// EncodeCert encode cert.
func EncodeCert(cert *x509.Certificate) (string, error) {
	var certOut bytes.Buffer
	if err := pem.Encode(&certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}); err != nil {
		return "", fmt.Errorf("failed to encode cert, detail-> %v", err)
	}

	return certOut.String(), nil
}

// BuildServerTLSConfigFromPath builds server tls config.
func BuildServerTLSConfigFromPath(caPath, certPath, keyPath string) (*tls.Config, error) {
	if caPath == "" || certPath == "" || keyPath == "" {
		return nil, fmt.Errorf("load server tls config failed, ca|servercert|serverkey path can't be empty")
	}

	caCertFile, err := LoadCertFile(caPath)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertFile)

	certs, err := BuildTLSCertificateViaPath(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load server certificate, %v", err.Error())
	}

	config := &tls.Config{
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certs,
	}
	return config, nil
}

// BuildServerTLSConfig builds server tls config.
func BuildServerTLSConfig(caCert *x509.Certificate, cert *x509.Certificate, key *rsa.PrivateKey) (*tls.Config, error) {
	if caCert == nil || cert == nil || key == nil {
		return nil, fmt.Errorf("load client tls config failed, ca|servercert|serverkey can't be empty")
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	certs := BuildTLSCertificate(cert, key)
	config := &tls.Config{
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certs,
	}
	return config, nil
}

// BuildClientTLSConfigViaPath builds client tls config.
func BuildClientTLSConfigViaPath(caPath, certPath, keyPath string) (*tls.Config, error) {
	if caPath == "" || certPath == "" || keyPath == "" {
		return nil, fmt.Errorf("load client tls config failed, ca|clientcert|clientkey path can't be empty")
	}

	caCertFile, err := LoadCertFile(caPath)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertFile)

	certs, err := BuildTLSCertificateViaPath(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load client certificate, %v", err.Error())
	}
	config := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: certs,
	}
	return config, nil
}

// BuildClientTLSConfig builds client tls config.
func BuildClientTLSConfig(caCert *x509.Certificate, cert *x509.Certificate, key *rsa.PrivateKey) (*tls.Config, error) {
	if caCert == nil || cert == nil || key == nil {
		return nil, fmt.Errorf("load client tls config failed, ca|clientcert|clientkey path can't be empty")
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	certs := BuildTLSCertificate(cert, key)
	config := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: certs,
	}
	return config, nil
}

// BuildTLSCertificateViaPath builds tls certificate.
func BuildTLSCertificateViaPath(certPath, keyPath string) ([]tls.Certificate, error) {
	certPEMBlock, err := LoadCertFile(certPath)
	if err != nil {
		return nil, err
	}

	keyPEMBlock, err := LoadCertFile(keyPath)
	if err != nil {
		return nil, err
	}

	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}
	return certs, nil
}

// BuildTLSCertificate builds tls certificate.
func BuildTLSCertificate(cert *x509.Certificate, key *rsa.PrivateKey) []tls.Certificate {
	certs := make([]tls.Certificate, 1)
	certs[0] = tls.Certificate{
		Certificate: [][]byte{
			cert.Raw,
		},
		PrivateKey: key,
		Leaf:       cert,
	}
	return certs
}

// LoadCertFile loads cert file.
func LoadCertFile(name string) ([]byte, error) {
	certContent, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("error reading %v certificate, %v", name, err.Error())
	}
	return certContent, nil
}

// DecodeCert loads cert from string content
func DecodeCert(certContent []byte) (*x509.Certificate, error) {
	certBlock, _ := pem.Decode(certContent)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate, detail-> %v", err)
	}
	return cert, nil
}
