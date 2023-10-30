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
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	cmerrorcode "github.com/secretflow/kuscia/pkg/confmanager/errorcode"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

const (
	KeyTypeForPCKS1 = "PKCS#1"
	KeyTypeForPCKS8 = "PKCS#8"
)

// ICertificateService is service which manager x509 keys and certificates. It can be used by grpc-mtls/https/process-call.
type ICertificateService interface {
	// ValidateGenerateKeyCertsRequest check request.
	ValidateGenerateKeyCertsRequest(ctx context.Context, request *confmanager.GenerateKeyCertsRequest) *errorcode.Errs
	// GenerateKeyCerts create a pair of x509 key and cert with domain ca cert.
	GenerateKeyCerts(context.Context, *confmanager.GenerateKeyCertsRequest) *confmanager.GenerateKeyCertsResponse
}

type certificateService struct {
	certValue  *atomic.Value
	privateKey *rsa.PrivateKey
}

type CertConfig struct {
	CertValue  *atomic.Value
	PrivateKey *rsa.PrivateKey
}

func NewCertificateService(config CertConfig) (ICertificateService, error) {
	return &certificateService{
		certValue:  config.CertValue,
		privateKey: config.PrivateKey,
	}, nil
}

func (s *certificateService) GenerateKeyCerts(ctx context.Context, request *confmanager.GenerateKeyCertsRequest) *confmanager.GenerateKeyCertsResponse {

	if errs := s.ValidateGenerateKeyCertsRequest(ctx, request); errs != nil {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrRequestInvalidate, errs.String()),
		}
	}

	if s.certValue == nil || s.certValue.Load() == nil {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "can not find domain cert"),
		}
	}
	domainCert, ok := s.certValue.Load().(*x509.Certificate)
	if !ok {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "domain cert is not valid"),
		}
	}

	s.normalizeRequest(request)
	subject := s.makeSubject(request)
	certTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	if request.DurationSec > 0 {
		certTmpl.NotAfter = time.Now().Add(time.Duration(request.DurationSec) * time.Second)
	}

	key, cert, err := tlsutils.GenerateX509KeyPairStruct(domainCert, s.privateKey, certTmpl)
	if err != nil {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "build key certs failed"),
		}
	}
	certEncoded, err := tlsutils.EncodeCert(cert)
	if err != nil {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "build key certs failed"),
		}
	}
	var keyEncoded string
	switch request.KeyType {
	case KeyTypeForPCKS1:
		keyEncoded, err = tlsutils.EncodeRsaKeyToPKCS1(key)
		if err != nil {
			return &confmanager.GenerateKeyCertsResponse{
				Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "build key certs failed"),
			}
		}
	case KeyTypeForPCKS8:
		keyEncoded, err = tlsutils.EncodeRsaKeyToPKCS8(key)
		if err != nil {
			return &confmanager.GenerateKeyCertsResponse{
				Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "build key certs failed"),
			}
		}
	default:
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "not implemented key encoded form"),
		}
	}

	domainCertEncoded, err := tlsutils.EncodeCert(domainCert)
	if err != nil {
		return &confmanager.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(cmerrorcode.ErrGenerateKeyCerts, "build key certs failed"),
		}
	}

	return &confmanager.GenerateKeyCertsResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Key:    base64.StdEncoding.EncodeToString([]byte(keyEncoded)),
		CertChain: []string{base64.StdEncoding.EncodeToString([]byte(certEncoded)),
			base64.StdEncoding.EncodeToString([]byte(domainCertEncoded))},
	}
}

func (s *certificateService) ValidateGenerateKeyCertsRequest(ctx context.Context, request *confmanager.GenerateKeyCertsRequest) *errorcode.Errs {
	var errs errorcode.Errs
	if request.CommonName == "" {
		errs.AppendErr(fmt.Errorf("common name must not be empty"))
	}
	if request.DurationSec < 0 {
		errs.AppendErr(fmt.Errorf("duration sedconds must greater than 0"))
	}
	if request.KeyType != "" && request.KeyType != KeyTypeForPCKS1 && request.KeyType != KeyTypeForPCKS8 {
		errs.AppendErr(fmt.Errorf("key tye must be [PKCS#1, PKCS#8]"))
	}
	if len(errs) == 0 {
		return nil
	}
	return &errs
}

func (s *certificateService) normalizeRequest(request *confmanager.GenerateKeyCertsRequest) {
	if request.DurationSec == 0 {
		request.DurationSec = 24 * 60 * 60
	}
	if request.KeyType == "" {
		request.KeyType = KeyTypeForPCKS1
	}
}

func (s *certificateService) makeSubject(request *confmanager.GenerateKeyCertsRequest) pkix.Name {
	subject := pkix.Name{
		CommonName: request.CommonName,
	}
	if request.Country != "" {
		subject.Country = []string{request.Country}
	}
	if request.Organization != "" {
		subject.Organization = []string{request.Organization}
	}
	if request.OrganizationUnit != "" {
		subject.OrganizationalUnit = []string{request.OrganizationUnit}
	}
	if request.Locality != "" {
		subject.Locality = []string{request.Locality}
	}
	if request.Province != "" {
		subject.Province = []string{request.Province}
	}
	if request.StreetAddress != "" {
		subject.StreetAddress = []string{request.StreetAddress}
	}
	return subject
}
