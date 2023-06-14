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
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAndLoadX509KeyPair(t *testing.T) {
	rootDir := t.TempDir()
	caCertFile := filepath.Join(rootDir, "ca.crt")
	caKeyFile := filepath.Join(rootDir, "ca.key")
	assert.NoError(t, CreateCAFile("testca", caCertFile, caKeyFile))

	caCert, caKey, err := LoadX509KeyPair(caCertFile, caKeyFile)
	assert.NoError(t, err)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{CommonName: "Test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certFile := filepath.Join(rootDir, "test.crt")
	keyFile := filepath.Join(rootDir, "test.key")

	certOut, err := os.Create(certFile)
	keyOut, err := os.Create(keyFile)
	assert.NoError(t, GenerateX509KeyPair(caCert, caKey, cert, certOut, keyOut))

	_, _, err = LoadX509KeyPair(certFile, keyFile)
	assert.NoError(t, err)
}
