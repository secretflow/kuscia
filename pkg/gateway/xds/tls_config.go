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

package xds

import (
	"fmt"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/gateway/utils"
)

type TLSCert struct {
	CertData string
	KeyData  string
	CAData   string
}

func GenerateUpstreamTLSConfigByCert(cert *TLSCert) (*core.TransportSocket, error) {
	return GenerateUpstreamTLSConfigByCertStr([]byte(cert.CertData), []byte(cert.KeyData), []byte(cert.CAData))
}

func GenerateUpstreamTLSConfigByCertStr(cert, key, ca []byte) (*core.TransportSocket, error) {
	if err := checkTLSConfig(cert, key, ca); err != nil {
		return nil, err
	}
	return generateUpstreamTLSConfig(cert, key, ca)
}

func GenerateDownstreamTLSConfigByCert(cert *TLSCert) (*core.TransportSocket, error) {
	return GenerateDownstreamTLSConfigByCertStr([]byte(cert.CertData), []byte(cert.KeyData), []byte(cert.CAData))
}

func GenerateDownstreamTLSConfigByCertStr(cert, key, ca []byte) (*core.TransportSocket, error) {
	if len(cert) == 0 || len(key) == 0 {
		return nil, fmt.Errorf("invalid downstream tls config, cert:(%s) key:(%s)", cert, key)
	}
	if err := checkTLSConfig(cert, key, ca); err != nil {
		return nil, err
	}
	return generateDownstreamTLSConfig(cert, key, ca)
}

func generateUpstreamTLSConfig(cert, key, ca []byte) (*core.TransportSocket, error) {
	tlsContext := &tls.UpstreamTlsContext{
		CommonTlsContext: &tls.CommonTlsContext{},
	}

	if len(cert) != 0 {
		tlsContext.CommonTlsContext.TlsCertificates = []*tls.TlsCertificate{
			{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: string(cert),
					},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: string(key),
					},
				},
			},
		}
	}

	if len(ca) != 0 {
		tlsContext.CommonTlsContext.ValidationContextType = &tls.CommonTlsContext_ValidationContext{
			ValidationContext: &tls.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: string(ca),
					},
				},
			},
		}
	}

	conf, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf("marshal UpstreamTlsContext failed with %s", err.Error())
	}
	return generateTransportSocket(conf), nil
}

func generateDownstreamTLSConfig(cert, key, ca []byte) (*core.TransportSocket, error) {
	tlsContext := &tls.DownstreamTlsContext{
		CommonTlsContext: &tls.CommonTlsContext{
			TlsCertificates: []*tls.TlsCertificate{
				{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineString{
							InlineString: string(cert),
						},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineString{
							InlineString: string(key),
						},
					},
				},
			},
		},
	}

	if len(ca) != 0 {
		tlsContext.RequireClientCertificate = &wrappers.BoolValue{
			Value: true,
		}
		tlsContext.CommonTlsContext.ValidationContextType = &tls.CommonTlsContext_ValidationContext{
			ValidationContext: &tls.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: string(ca),
					},
				},
			},
		}
	}
	conf, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf("marshal UpstreamTlsContext failed with %s", err.Error())
	}
	return generateTransportSocket(conf), nil
}

func checkTLSConfig(cert, key, ca []byte) error {
	if len(cert) != 0 {
		if !utils.VerifyCert(cert) {
			return fmt.Errorf("invalid cert data")
		}
		if !utils.VerifySSLKey(key) {
			return fmt.Errorf("invalid cert key")
		}
	}

	if len(ca) != 0 {
		if !utils.VerifyCert(ca) {
			return fmt.Errorf("invalid ca data")
		}
	}
	return nil
}

func generateTransportSocket(conf *anypb.Any) *core.TransportSocket {
	return &core.TransportSocket{
		Name: "envoy.transport_sockets.tls",
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: conf,
		},
	}
}
