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
	"context"
	cryptrand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	"github.com/secretflow/kuscia/pkg/gateway/xds"
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
	fakeToken         = "FwBvarrLUpfACr00v8AiIbHbFcYguNqvu92XRJ2YysU="
)

func newDomainRouteTestInfo(namespace string) *DomainRouteTestInfo {
	kusciaClient := kusciaFake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, controller.NoResyncPeriodFunc())
	domainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()

	// parse rsa private key
	priKey, err := rsa.GenerateKey(cryptrand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
	token, err := base64.StdEncoding.DecodeString(fakeToken)
	if err != nil {
		log.Fatal(err)
	}

	fakeRevisionToken, err = encryptToken(&priKey.PublicKey, token)
	if err != nil {
		log.Fatal(err)
	}

	config := &DomainRouteConfig{
		Namespace:     namespace,
		Prikey:        priKey,
		HandshakePort: 1054,
	}
	c := NewDomainRouteController(config, fake.NewSimpleClientset(), kusciaClient, domainRouteInformer)
	kusciaInformerFactory.Start(wait.NeverStop)

	return &DomainRouteTestInfo{
		c,
		kusciaClient,
	}
}

type DrTestCase struct {
	dr               *kusciaapisv1alpha1.DomainRoute
	token            string
	expectedResponse string
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
				t.Fatal("add route failed")
			}
			if !add && err == nil {
				t.Fatal("delete route failed")
			}
		} else { // intbound
			find := false
			tokenAuth, err := xds.GetTokenAuth()
			if err != nil {
				t.Fatalf("get token auth fail: %v", err)
			}
			for _, sourceTokens := range tokenAuth.SourceTokenList {
				if sourceTokens.Source == tc.dr.Spec.Source {
					for _, token := range sourceTokens.Tokens {
						if token == tc.token {
							find = true
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
			clusterName := fmt.Sprintf("%s-%s", vhName, string(tc.dr.Spec.Endpoint.Ports[0].Protocol))
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
			tokenAuth, err := xds.GetTokenAuth()
			if err != nil {
				t.Fatalf("get token auth fail: %v", err)
			}
			for _, sourceTokens := range tokenAuth.SourceTokenList {
				if sourceTokens.Source == tc.dr.Spec.Source {
					for _, token := range sourceTokens.Tokens {
						if token == NoopToken {
							find = true
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
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
	time.Sleep(200 * time.Millisecond)

	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&c.prikey.PublicKey),
	}
	pubPemData := pem.EncodeToMemory(block)

	var testcases = []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rsa-inbound",
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "test",
					Destination: "default",
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
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "default",
					Destination: "test",
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

func TestTokenRand(t *testing.T) {
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
	time.Sleep(200 * time.Millisecond)

	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	token := base64.StdEncoding.EncodeToString(b)

	var testcases = []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rand-inbound",
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "test",
					Destination: "default",
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
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRAND,
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: token,
							},
						},
					},
				},
			},
			token: token,
		},
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rand-outbound",
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "default",
					Destination: "test",
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
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRAND,
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: token,
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
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
	time.Sleep(1000 * time.Millisecond)

	realInternalServer := InternalServer
	defer func() {
		InternalServer = realInternalServer
	}()

	InternalServer = "http://localhost:1054"
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&c.prikey.PublicKey),
	}
	pubPemData := pem.EncodeToMemory(block)
	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "handshake",
			Namespace: "default",
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:      "default",
			Destination: "default",
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: "127.0.0.1",
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     1054,
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
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
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
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "default",
					Destination: "mutual",
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: EnvoyServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								IsTLS:    true,
								Port:     1443,
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
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "mutual",
					Destination: "default",
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Host: EnvoyServerIP,
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
								IsTLS:    true,
								Port:     443,
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
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
	time.Sleep(100 * time.Millisecond)

	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	token := base64.StdEncoding.EncodeToString(b)

	testcases := []DrTestCase{
		{
			dr: &kusciaapisv1alpha1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rand-outbound",
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "default",
					Destination: "test",
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
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRAND,
					},
				},
				Status: kusciaapisv1alpha1.DomainRouteStatus{
					TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
						Tokens: []kusciaapisv1alpha1.DomainRouteToken{
							{
								Token: token,
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
					Namespace: "default",
				},
				Spec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:      "default",
					Destination: "foo",
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
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRAND,
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
								Token: token,
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
	c := newDomainRouteTestInfo("default")
	stopCh := make(chan struct{})
	go c.Run(1, stopCh)
	time.Sleep(200 * time.Millisecond)

	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	token := base64.StdEncoding.EncodeToString(b)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "append-headers",
			Namespace: "default",
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:      "test",
			Destination: "default",
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
				TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRAND,
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
						Token: token,
					},
				},
			},
		},
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	decorator, err := xds.GetHeaderDecorator()
	if err != nil {
		log.Fatal("get header decorator filter fail")
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
					log.Fatalf("expected header entry(%s, %s) unexist", k, v)
				}
			}
			foundSource = true
			break
		}
	}
	if !foundSource {
		log.Fatalf("add sourceHeader(test) fail")
	}

	c.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Delete(context.Background(), dr.Name, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)

	decorator, err = xds.GetHeaderDecorator()
	if err != nil {
		log.Fatal("get header decorator filter fail")
	}
	foundSource = false
	for _, entry := range decorator.AppendHeaders {
		if entry.Source == "test" {
			foundSource = true
		}
	}
	if foundSource {
		log.Fatalf("deleted sourceHeader(test) fail")
	}

	stopCh <- struct{}{}
	time.Sleep(time.Second)
}
