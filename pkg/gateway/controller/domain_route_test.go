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

package controller

import (
	"bytes"
	"context"
	cryptrand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	envoyroute "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/controller"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciaFake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
)

type DomainRouteTestInfo struct {
	*DomainRouteController
	client versioned.Interface
}

const (
	FakeServerIP   = "127.0.0.1"
	FakeServerPort = 9999

	EnvoyServerIP      = "127.0.0.1"
	ExternalServerPort = 10081
)

var (
	fakeRevisionToken string
	pubPemData        []byte
	fakeToken         = "FwBvarrLUpfACr00v8AiIbHbFcYguNqvu92XRJ2YysU="
)

func newDomainRouteTestInfo(namespace string, port uint32) *DomainRouteTestInfo {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	kusciaClient := kusciaFake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, controller.NoResyncPeriodFunc())
	domainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()

	// parse rsa private key
	priKey, err := rsa.GenerateKey(cryptrand.Reader, 2048)
	if err != nil {
		nlog.Fatal(err)
	}
	token, err := base64.StdEncoding.DecodeString(fakeToken)
	if err != nil {
		nlog.Fatal(err)
	}

	fakeRevisionToken, err = encryptToken(&priKey.PublicKey, token)
	if err != nil {
		nlog.Fatal(err)
	}

	caKey, caCertBytes, err := tlsutils.CreateCA(namespace)
	if err != nil {
		nlog.Fatal(err)
	}
	caPEM := new(bytes.Buffer)
	if err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	}); err != nil {
		nlog.Fatal(err)
	}
	caCert, err := tlsutils.ParseCert(caPEM.Bytes(), "")
	if err != nil {
		nlog.Fatal(err)
	}
	config := &DomainRouteConfig{
		MasterConfig: &config.MasterConfig{
			Namespace: "kuscia",
		},
		Namespace:     namespace,
		Prikey:        priKey,
		HandshakePort: port,
		CAKey:         caKey,
		CACert:        caCert,
	}
	c := NewDomainRouteController(config, fake.NewSimpleClientset(), kusciaClient, domainRouteInformer)
	kusciaInformerFactory.Start(wait.NeverStop)
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&c.prikey.PublicKey),
	}
	pubPemData = pem.EncodeToMemory(block)
	return &DomainRouteTestInfo{
		c,
		kusciaClient,
	}
}

type DrTestCase struct {
	dr    *kusciaapisv1alpha1.DomainRoute
	token string
}

func (c *DomainRouteTestInfo) runPlainCase(cases []DrTestCase, t *testing.T, add bool) {
	for _, tc := range cases {
		if add {
			c.client.KusciaV1alpha1().DomainRoutes(tc.dr.Namespace).Create(context.Background(), tc.dr, metav1.CreateOptions{})
		} else {
			c.client.KusciaV1alpha1().DomainRoutes(tc.dr.Namespace).Delete(context.Background(), tc.dr.Name,
				metav1.DeleteOptions{})
		}
		time.Sleep(200 * time.Millisecond) // wait for envoy to sync config

		if tc.dr.Spec.Source == tc.dr.Namespace { // outbound
			vhName := fmt.Sprintf("%s-to-%s", tc.dr.Spec.Source, tc.dr.Spec.Destination)
			_, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
			if add && err != nil {
				t.Fatal("add route failed ", err)
			}
			if !add && err == nil {
				t.Fatal("delete route failed")
			}
		} else { // intbound
			find := false
			tokenAuth, _ := xds.GetTokenAuth()
			if tokenAuth != nil {
				for _, sourceTokens := range tokenAuth.SourceTokenList {
					if sourceTokens.Source == tc.dr.Spec.Source {
						for _, token := range sourceTokens.Tokens {
							if token == tc.token {
								find = true
							}
						}
					}
				}
			}
			if add && !find {
				t.Fatal("set token auth failed")
			}
			if !add && find {
				t.Fatal("delete token auth failed")
			}
		}
	}
}

func (c *DomainRouteTestInfo) runMTLSCase(cases []DrTestCase, t *testing.T, add bool) {
	for _, tc := range cases {
		if add {
			c.client.KusciaV1alpha1().DomainRoutes(tc.dr.Namespace).Create(context.Background(), tc.dr, metav1.CreateOptions{})
		} else {
			c.client.KusciaV1alpha1().DomainRoutes(tc.dr.Namespace).Delete(context.Background(), tc.dr.Name,
				metav1.DeleteOptions{})
		}
		time.Sleep(200 * time.Millisecond) // wait for envoy to sync config

		if tc.dr.Spec.Source == tc.dr.Namespace { // outbound
			vhName := fmt.Sprintf("%s-to-%s", tc.dr.Spec.Source, tc.dr.Spec.Destination)
			_, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
			if add && err != nil {
				t.Fatal("add route failed")
			}
			if !add && err == nil {
				t.Fatal("delete route failed")
			}
			clusterName := fmt.Sprintf("%s-%s", vhName, string(tc.dr.Spec.Endpoint.Ports[0].Name))
			cluster, err := xds.QueryCluster(clusterName)
			if add {
				if err != nil || cluster == nil {
					t.Fatal("add route fail")
				}
				var tlsContext tls.UpstreamTlsContext
				err = cluster.TransportSocket.GetTypedConfig().UnmarshalTo(&tlsContext)
				if err != nil {
					t.Fatal("add tls cluster fail")
				}
			}
		} else { // intbound
			find := false
			tokenAuth, _ := xds.GetTokenAuth()
			if tokenAuth != nil {
				for _, sourceTokens := range tokenAuth.SourceTokenList {
					if sourceTokens.Source == tc.dr.Spec.Source {
						for _, token := range sourceTokens.Tokens {
							if token == NoopToken {
								find = true
							}
						}
					}
				}
			}
			if add && !find {
				t.Fatal("set mtls token auth failed")
			}
			if !add && find {
				t.Fatal("delete mtls token auth failed")
			}
		}
	}
}

func TestTokenRSA(t *testing.T) {
	ns := "defaultrsa"
	c := newDomainRouteTestInfo(ns, 1054)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	var testcases = []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rsa-inbound",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            "test",
					Destination:       ns,
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: EnvoyServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								Port:     ExternalServerPort,
								Name:     "http",
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
						DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
						TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						RevisionToken: kusciaapisv1alpha1.DomainRouteToken{
							Token: fakeRevisionToken,
						},
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: fakeRevisionToken,
							},
						},
					},
				},
			},
			token: fakeToken,
		},
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rsa-outbound",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            ns,
					Destination:       "test",
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: FakeServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								Port:     FakeServerPort,
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
						DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
						TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						RevisionInitializer: "",
						RevisionToken: kusciaapisv1alpha1.DomainRouteToken{
							Token: fakeRevisionToken,
						},
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: fakeRevisionToken,
							},
						},
					},
				},
			},
		},
	}
	c.runPlainCase(testcases, t, true)
	c.runPlainCase(testcases, t, false)

	stopCh <- struct{}{}
	time.Sleep(time.Second)
}

func TestTokenHandshake(t *testing.T) {
	ns := "defaulthandshake"
	port := 11054
	c := newDomainRouteTestInfo(ns, uint32(port))
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(1000 * time.Millisecond)

	realInternalServer := utils.InternalServer
	defer func() {
		utils.InternalServer = realInternalServer
	}()

	utils.InternalServer = fmt.Sprintf("http://localhost:%d", port)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns + "-" + ns,
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       ns,
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: "127.0.0.1",
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     port,
						Name:     "http",
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				RevisionInitializer: c.gateway.Name,
				RevisionToken: kusciaapisv1alpha1.DomainRouteToken{
					IsReady: true,
				},
			},
		},
	}
	c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	// wait for handshake
	time.Sleep(1000 * time.Millisecond)

	if dr, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).Get(context.Background(), dr.Name,
		metav1.GetOptions{}); err != nil || dr.Status.TokenStatus.RevisionToken.Token == "" {
		t.Fatalf("handshake fail")
	}

	stopCh <- struct{}{}
	time.Sleep(time.Second)
}

func TestMutualAuth(t *testing.T) {
	ns := "defaultauth"

	c := newDomainRouteTestInfo(ns, 1051)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(100 * time.Millisecond)

	var sslKey, sslCrt []byte
	var err error

	rootDir := t.TempDir()
	certFile := filepath.Join(rootDir, "test.crt")
	keyFile := filepath.Join(rootDir, "test.key")
	assert.NoError(t, tlsutils.CreateCAFile("testca", certFile, keyFile))

	if sslKey, err = os.ReadFile(keyFile); err != nil {
		t.Fatalf("Read ssl key failed with (%s)", err.Error())
		return
	}
	if sslCrt, err = os.ReadFile(certFile); err != nil {
		t.Fatalf("Read ssl crt failed with (%s)", err.Error())
		return
	}
	testcases := []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mutual-outbound",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            ns,
					Destination:       "mutual",
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: EnvoyServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								IsTLS:    true,
								Port:     1443,
								Name:     "httptest",
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationMTLS,
					MTLSConfig: &kusciaapisv1alpha1.DomainRouteMTLSConfig{
						SourceClientPrivateKey: base64.StdEncoding.EncodeToString([]byte(sslKey)),
						SourceClientCert:       base64.StdEncoding.EncodeToString([]byte(sslCrt)),
					},
				},
			},
		},
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mutual-inbound",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            "mutual",
					Destination:       ns,
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: EnvoyServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								IsTLS:    true,
								Port:     443,
								Name:     "httptest",
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationMTLS,
					MTLSConfig: &kusciaapisv1alpha1.DomainRouteMTLSConfig{
						SourceClientPrivateKey: base64.StdEncoding.EncodeToString(sslKey),
						SourceClientCert:       base64.StdEncoding.EncodeToString(sslCrt),
					},
				},
			},
		},
	}
	c.runMTLSCase(testcases, t, true)
	c.runMTLSCase(testcases, t, false)

	stopCh <- struct{}{}
	time.Sleep(time.Second)
}

func TestTransit(t *testing.T) {
	ns := "defaulttransit"
	c := newDomainRouteTestInfo(ns, 1055)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(100 * time.Millisecond)

	testcases := []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rand-outbound",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            ns,
					Destination:       "test",
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: FakeServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								Port:     FakeServerPort,
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
						SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
						DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: fakeRevisionToken,
							},
						},
					},
				},
			},
		},
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "routing",
					Namespace: ns,
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:            ns,
					Destination:       "foo",
					InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: "test",
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
							},
						},
					},
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationNone,
					BodyEncryption: &kusciaapisv1alpha1.BodyEncryption{
						Algorithm: kusciaapisv1alpha1.BodyEncryptionAlgorithmSM4,
					},
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
						SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
						DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
					},
					Transit: &kusciaapisv1alpha1.Transit{
						Domain: &kusciaapisv1alpha1.DomainTransit{
							DomainID: "test",
						},
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: fakeRevisionToken,
							},
						},
					},
				},
			},
		},
	}
	c.runPlainCase(testcases, t, true)
	c.runPlainCase(testcases, t, false)
	stopCh <- struct{}{}
	time.Sleep(time.Second)
}

func TestAppendHeaders(t *testing.T) {
	ns := "defaultappend"
	c := newDomainRouteTestInfo(ns, 1056)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "append-headers",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            "test",
			Destination:       ns,
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
			RequestHeadersToAdd: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	decorator, err := xds.GetHeaderDecorator()
	if err != nil {
		nlog.Fatal("get header decorator filter fail")
	}

	foundSource := false
	for _, entry := range decorator.AppendHeaders {
		if entry.Source == "test" {
			for k, v := range dr.Spec.RequestHeadersToAdd {
				foundEntry := false
				for _, entry := range entry.Headers {
					if k == entry.Key && v == entry.Value {
						foundEntry = true
						break
					}
				}
				if !foundEntry {
					nlog.Fatalf("expected header entry(%s, %s) unexist", k, v)
				}
			}
			foundSource = true
			break
		}
	}
	if !foundSource {
		nlog.Fatalf("add sourceHeader(test) fail")
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Delete(context.Background(), dr.Name, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)

	decorator, _ = xds.GetHeaderDecorator()
	if decorator != nil {
		foundSource = false
		for _, entry := range decorator.AppendHeaders {
			if entry.Source == "test" {
				foundSource = true
			}
		}
		if foundSource {
			nlog.Fatalf("deleted sourceHeader(test) fail")
		}
	}

	stopCh <- struct{}{}
	time.Sleep(time.Second)
}

func TestGenerateInternalRoute(t *testing.T) {
	ns := "defaultinternal"
	c := newDomainRouteTestInfo(ns, 1057)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "interconn",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       "test",
			InterConnProtocol: kusciaapisv1alpha1.InterConnBFIA,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	vhName := fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	vh, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	routes := vh.GetRoutes()

	findPushPrefix := false
	findSchedulePrefix := false
	for _, route := range routes {
		val, ok := route.Match.PathSpecifier.(*envoyroute.RouteMatch_Prefix)
		if !ok {
			continue
		}
		if val.Prefix == "/v1/interconn/chan/push" {
			findPushPrefix = true
			assert.Equal(t, len(route.RequestHeadersToAdd), 3)
			assert.True(t, route.RequestHeadersToAdd[1].Header.Key == "x-ptp-source-inst-id")
			assert.True(t, route.RequestHeadersToAdd[2].Header.Key == "x-ptp-source-node-id")
		}
		if val.Prefix == "/v1/interconn/schedule/" {
			findSchedulePrefix = true
			assert.Equal(t, len(route.RequestHeadersToAdd), 1)
		}
		if findPushPrefix || findSchedulePrefix {
			assert.True(t, route.RequestHeadersToAdd[0].Header.Key == "x-interconn-protocol")
			assert.True(t, route.RequestHeadersToAdd[0].Header.Value == "bfia")
		}
	}
	assert.True(t, findSchedulePrefix)
	assert.True(t, findPushPrefix)

	dr.Spec.InterConnProtocol = kusciaapisv1alpha1.InterConnKuscia
	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Update(context.Background(), dr, metav1.UpdateOptions{})
	time.Sleep(200 * time.Millisecond)
	vh, err = xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	routes = vh.GetRoutes()
	assert.Equal(t, len(routes), 2)
	route := routes[0]
	val, ok := routes[0].Match.PathSpecifier.(*envoyroute.RouteMatch_Prefix)
	assert.True(t, ok)
	assert.NotNil(t, val)
	assert.True(t, route.RequestHeadersToAdd[0].Header.Key == "x-interconn-protocol")
	assert.True(t, route.RequestHeadersToAdd[0].Header.Value == "kuscia")
	assert.Equal(t, len(route.RequestHeadersToAdd), 4)
}

func TestCheckHealthy(t *testing.T) {
	ns := "defaultinternal"
	c := newDomainRouteTestInfo(ns, 1057)
	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "interconn",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       "test",
			InterConnProtocol: kusciaapisv1alpha1.InterConnBFIA,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}
	dr, err := c.client.KusciaV1alpha1().DomainRoutes(ns).Create(context.Background(), dr, metav1.CreateOptions{})
	assert.NoError(t, err)

	err = c.markDestUnreachable(context.Background(), dr)
	assert.NoError(t, err)

	_, ok := c.drHeartbeat[dr.Name]
	assert.True(t, ok)
	assert.False(t, dr.Status.IsDestinationUnreachable)
	beforeTm, _ := time.ParseDuration("-1m")
	c.drHeartbeat[dr.Name] = time.Now().Add(beforeTm)
	err = c.markDestUnreachable(context.Background(), dr)
	assert.NoError(t, err)
	dr, err = c.client.KusciaV1alpha1().DomainRoutes(ns).Get(context.Background(), dr.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, dr.Status.IsDestinationUnreachable)

	c.markDestReachable(context.Background(), dr)
	dr, err = c.client.KusciaV1alpha1().DomainRoutes(ns).Get(context.Background(), dr.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.False(t, dr.Status.IsDestinationUnreachable)
}

func TestPollerFilter(t *testing.T) {
	ns := "bob"
	c := newDomainRouteTestInfo(ns, 1057)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alice-bob",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            "alice",
			Destination:       "bob",
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
			Transit: &kusciaapisv1alpha1.Transit{
				TransitMethod: kusciaapisv1alpha1.TransitMethodReverseTunnel,
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	f, err := xds.GetHTTPFilterConfig(xds.PollerFilterName, xds.InternalListener)
	assert.NoError(t, err)
	assert.NotNil(t, f)

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Delete(context.Background(), dr.Name, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)
	f, err = xds.GetHTTPFilterConfig(xds.PollerFilterName, xds.InternalListener)
	assert.Error(t, err)
}

func TestReceiverFilter(t *testing.T) {
	ns := "alice"
	c := newDomainRouteTestInfo(ns, 1057)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alice-bob",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       "bob",
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
			Transit: &kusciaapisv1alpha1.Transit{
				TransitMethod: kusciaapisv1alpha1.TransitMethodReverseTunnel,
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	vhName := fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	vh, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	var dvh *envoyroute.Route
	for _, v := range vh.GetRoutes() {
		if v.Name == "default" {
			dvh = v
		}
	}

	assert.NotNil(t, dvh)
	assert.NotNil(t, dvh.GetRoute())
	assert.Equal(t, "envoy-cluster", dvh.GetRoute().ClusterSpecifier.(*envoyroute.RouteAction_Cluster).Cluster)

	f, err := xds.GetHTTPFilterConfig(xds.ReceiverFilterName, xds.InternalListener)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	f, err = xds.GetHTTPFilterConfig(xds.ReceiverFilterName, xds.ExternalListener)
	assert.NoError(t, err)
	assert.NotNil(t, f)

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Delete(context.Background(), dr.Name, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)

	f, err = xds.GetHTTPFilterConfig(xds.ReceiverFilterName, xds.InternalListener)
	assert.Error(t, err)
	f, err = xds.GetHTTPFilterConfig(xds.ReceiverFilterName, xds.ExternalListener)
	assert.Error(t, err)
	vh, err = xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.Error(t, err)
}

func TestDeleteRoute(t *testing.T) {
	ns := "alice"
	c := newDomainRouteTestInfo(ns, 1057)
	stopCh := make(chan struct{})
	go c.Run(context.Background(), 1, stopCh)
	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alice-bob",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       "bob",
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	vhName := fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	vh, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	routes := vh.GetRoutes()

	assert.Equal(t, "default", routes[0].Name)
	err = xds.DeleteRoute("default", vhName, xds.InternalRoute)
	assert.NoError(t, err)

	vh, err = xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	var dvh *envoyroute.Route
	for _, v := range vh.GetRoutes() {
		if v.Name == "default" {
			dvh = v
		}
	}
	assert.Nil(t, dvh)
}
