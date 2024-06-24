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

package service

import (
	"context"
	"crypto/x509"
	"sync/atomic"
	"testing"

	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/asserts"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/stretchr/testify/assert"
)

func newNewCertificateService() (ICertificateService, error) {
	key, certBytes, err := tls.CreateCA("test")
	asserts.NotNil(err, "create ca failed")
	cert, err := x509.ParseCertificate(certBytes)
	asserts.NotNil(err, "parse ca cert failed")
	dominaCertValue := &atomic.Value{}
	dominaCertValue.Store(cert)
	return NewCertificateService(CertConfig{
		PrivateKey: key,
		CertValue:  dominaCertValue,
	})
}

func TestNewCertificateService(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	asserts.NotNil(err, "new certificate service failed")
	asserts.IsNil(certService, "new certificate service return nil")
}

func Test_certificateService_GenerateKeyCerts_PKCS1(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	assert.Nil(t, err)
	assert.NotNil(t, certService)
	got := certService.GenerateKeyCerts(context.Background(), &confmanager.GenerateKeyCertsRequest{
		CommonName: "test",
		KeyType:    KeyTypeForPCKS1,
	})

	assert.Equal(t, 0, int(got.Status.Code))
	assert.NotEmpty(t, got.Key)
	assert.Equal(t, int(2), len(got.CertChain))
}

func Test_certificateService_GenerateKeyCerts_PKCS8(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	assert.Nil(t, err)
	assert.NotNil(t, certService)
	got := certService.GenerateKeyCerts(context.Background(), &confmanager.GenerateKeyCertsRequest{
		CommonName: "test",
		KeyType:    KeyTypeForPCKS8,
	})

	assert.Equal(t, 0, int(got.Status.Code))
	assert.NotEmpty(t, got.Key)
	assert.Equal(t, 2, len(got.CertChain))
}

func Test_certificateService_ValidateGenerateKeyCertsRequest_PKCS1_Error(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	assert.Nil(t, err)
	assert.NotNil(t, certService)

	got := certService.ValidateGenerateKeyCertsRequest(context.Background(), &confmanager.GenerateKeyCertsRequest{
		CommonName: "test",
		KeyType:    KeyTypeForPCKS1,
	})

	assert.Nil(t, got)
}

func Test_certificateService_ValidateGenerateKeyCertsRequest_PKCS1(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	assert.Nil(t, err)
	assert.NotNil(t, certService)

	got := certService.ValidateGenerateKeyCertsRequest(context.Background(), &confmanager.GenerateKeyCertsRequest{
		KeyType: KeyTypeForPCKS1,
	})

	assert.NotNil(t, got)
	assert.Equal(t, 1, len(*got))
}

func Test_certificateService_ValidateGenerateKeyCertsRequest_3(t *testing.T) {
	t.Parallel()
	certService, err := newNewCertificateService()
	assert.Nil(t, err)
	assert.NotNil(t, certService)

	got := certService.ValidateGenerateKeyCertsRequest(context.Background(), &confmanager.GenerateKeyCertsRequest{
		KeyType:     "123",
		DurationSec: -123,
	})

	assert.NotNil(t, got)
	assert.Equal(t, 3, len(*got))
}
