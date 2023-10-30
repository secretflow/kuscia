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
	tests := []struct {
		name string
		want ICertificateService
	}{
		{
			name: "cm new certificate service should success",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certService, err := newNewCertificateService()
			asserts.NotNil(err, "new certificate service failed")
			asserts.IsNil(certService, "new certificate service return nil")
		})
	}
}

func Test_certificateService_GenerateKeyCerts(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.GenerateKeyCertsRequest
	}
	tests := []struct {
		name             string
		args             args
		wantCode         int32
		wantCertChainLen int
	}{
		{
			name: "cm generate pkcs#1 key certs should return success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.GenerateKeyCertsRequest{
					CommonName: "test",
					KeyType:    KeyTypeForPCKS1,
				},
			},
			wantCode:         0,
			wantCertChainLen: 2,
		},
		{
			name: "cm generate pkcs#8 key certs should return success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.GenerateKeyCertsRequest{
					CommonName: "test",
					KeyType:    KeyTypeForPCKS8,
				},
			},
			wantCode:         0,
			wantCertChainLen: 2,
		},
	}
	for _, tt := range tests {
		certService, err := newNewCertificateService()
		asserts.NotNil(err, "new certificate service failed")
		asserts.IsNil(certService, "new certificate service return nil")
		t.Run(tt.name, func(t *testing.T) {
			got := certService.GenerateKeyCerts(tt.args.ctx, tt.args.request)
			if got.Status.Code != tt.wantCode {
				t.Errorf("GenerateKeyCerts() = %v, wantCode %v", got, tt.wantCode)
			}
			if got.Key == "" {
				t.Errorf("GenerateKeyCerts() = %v, key empty", got)
			}
			if len(got.CertChain) != tt.wantCertChainLen {
				t.Errorf("GenerateKeyCerts() = %v, cert chain len %v", got, tt.wantCertChainLen)
			}
		})
	}
}

func Test_certificateService_ValidateGenerateKeyCertsRequest(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.GenerateKeyCertsRequest
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantErrLen int
	}{
		{
			name: "cm generate key certs request validate should return success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.GenerateKeyCertsRequest{
					CommonName: "test",
					KeyType:    KeyTypeForPCKS1,
				},
			},
			wantErr:    false,
			wantErrLen: 0,
		},
		{
			name: "cm generate key certs request validate should return 1 error",
			args: args{
				ctx: context.Background(),
				request: &confmanager.GenerateKeyCertsRequest{
					KeyType: KeyTypeForPCKS1,
				},
			},
			wantErr:    true,
			wantErrLen: 1,
		},
		{
			name: "cm generate key certs request validate  should return 3 error",
			args: args{
				ctx: context.Background(),
				request: &confmanager.GenerateKeyCertsRequest{
					KeyType:     "123",
					DurationSec: -123,
				},
			},
			wantErr:    true,
			wantErrLen: 3,
		},
	}
	for _, tt := range tests {
		certService, err := newNewCertificateService()
		asserts.NotNil(err, "new certificate service failed")
		asserts.IsNil(certService, "new certificate service return nil")
		t.Run(tt.name, func(t *testing.T) {
			got := certService.ValidateGenerateKeyCertsRequest(tt.args.ctx, tt.args.request)
			if (got != nil) != tt.wantErr {
				t.Errorf("ValidateGenerateKeyCertsRequest() = %v, wantErr %v", got, tt.wantErr)
			}
			if got != nil {
				if len(*got) != tt.wantErrLen {
					t.Errorf("ValidateGenerateKeyCertsRequest() = %v, wantErr %v", got, tt.wantErr)
				}
			}
		})
	}
}
