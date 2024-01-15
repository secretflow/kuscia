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

//nolint:dulp
package clusterdomainroute

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/scheme"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func Test_compareSpec(t *testing.T) {
	testcdr := &kusciaapisv1alpha1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"auth": "test",
			},
		},
		Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
			DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
				AuthenticationType: "Token",
			},
		},
	}
	testdr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"auth": "test",
			},
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			AuthenticationType: "Token",
		},
	}
	assert.True(t, compareSpec(testcdr, testdr))
	testcdr.Labels["auth2"] = "test"
	assert.False(t, compareSpec(testcdr, testdr))
	delete(testcdr.Labels, "auth2")
	assert.True(t, compareSpec(testcdr, testdr))
	testcdr.Spec.Destination = "alice"
	assert.False(t, compareSpec(testcdr, testdr))
	testcdr.Spec.Destination = ""
	assert.True(t, compareSpec(testcdr, testdr))
}

func createCrtString(t *testing.T) string {
	rootDir := t.TempDir()
	caCertFile := filepath.Join(rootDir, "ca.crt")
	caKeyFile := filepath.Join(rootDir, "ca.key")
	assert.NoError(t, tls.CreateCAFile("testca", caCertFile, caKeyFile))
	f, err := os.Open(caCertFile)
	assert.NoError(t, err)
	testCrt, err := io.ReadAll(f)
	assert.NoError(t, err)
	return base64.StdEncoding.EncodeToString(testCrt)
}

func NewTestController() *controller {
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeClient := kubefake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})
	ic := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})

	c := ic.(*controller)
	return c
}

func Test_controller_add_label(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	chStop := make(chan struct{})
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeClient := kubefake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})

	ctx := signals.NewKusciaContextWithStopCh(chStop)
	ic := NewController(ctx, controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})
	alice := "aliceaddlabel"
	bob := "bobaddlabel"
	charlie := "charlieaddlabel"
	go func() {
		time.Sleep(300 * time.Millisecond)
		var err error
		certstr := createCrtString(t)
		aliceDomain := &kusciaapisv1alpha1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice,
			},
			Spec: kusciaapisv1alpha1.DomainSpec{
				Cert: certstr,
			},
		}
		bobDomain := &kusciaapisv1alpha1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: bob,
			},
			Spec: kusciaapisv1alpha1.DomainSpec{
				Cert: certstr,
			},
		}
		charlieDomain := &kusciaapisv1alpha1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: charlie,
			},
			Spec: kusciaapisv1alpha1.DomainSpec{
				Cert: certstr,
			},
		}
		_, err = kusciaClient.KusciaV1alpha1().Domains().Create(ctx, aliceDomain, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().Gateways(alice).Create(ctx, &kusciaapisv1alpha1.Gateway{
			Status: kusciaapisv1alpha1.GatewayStatus{
				NetworkStatus: []kusciaapisv1alpha1.GatewayEndpointStatus{
					{
						Type: common.GenerateClusterName(alice, bob, "http"),
						Name: "DomainRoute",
					},
					{
						Type: common.GenerateClusterName(alice, bob, "http"),
						Name: "DomainRoute",
					},
				},
			},
		}, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().Domains().Create(ctx, bobDomain, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().Domains().Create(ctx, charlieDomain, metav1.CreateOptions{})
		assert.NoError(t, err)
		testdr := &kusciaapisv1alpha1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + bob,
			},
			Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
				DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRSA,
					},
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Name: "http",
							},
						},
					},
				},
			},
			Status: kusciaapisv1alpha1.ClusterDomainRouteStatus{},
		}
		testdr2 := &kusciaapisv1alpha1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + charlie,
				Labels: map[string]string{
					"auth": "test",
				},
			},
			Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
				DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:             alice,
					Destination:        charlie,
					AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
					Endpoint: kusciaapisv1alpha1.DomainEndpoint{
						Ports: []kusciaapisv1alpha1.DomainPort{
							{
								Name: "http",
							},
						},
					},
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRSA,
					},
				},
			},
			Status: kusciaapisv1alpha1.ClusterDomainRouteStatus{},
		}
		nlog.Info("create ", testdr.Name, " ", testdr2.Name)
		_, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testdr, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testdr2, metav1.CreateOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		close(chStop)
	}()
	ic.Run(4)
	ic.Stop()
	time.Sleep(100 * time.Millisecond)
	dr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(alice).Get(ctx, alice+"-"+bob, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, alice, dr.Labels[common.KusciaSourceKey])
	assert.Equal(t, bob, dr.Labels[common.KusciaDestinationKey])
	dr2, err := kusciaClient.KusciaV1alpha1().DomainRoutes(alice).Get(ctx, alice+"-"+charlie, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, alice, dr2.Labels[common.KusciaSourceKey])
	assert.Equal(t, charlie, dr2.Labels[common.KusciaDestinationKey])
}

func Test_controller_update_label(t *testing.T) {
	c := NewTestController()
	sourceDomain := "alice"
	destDomain := "bob"

	testdr1 := &kusciaapisv1alpha1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: sourceDomain + "-" + destDomain,
			Labels: map[string]string{
				"l4": "v4",
				"l5": "v5",
				"l1": "v0",
			},
		},
		Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
			DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
				Source:             sourceDomain,
				Destination:        destDomain,
				AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
				TokenConfig: &kusciaapisv1alpha1.TokenConfig{
					TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRSA,
				},
			},
		},
		Status: kusciaapisv1alpha1.ClusterDomainRouteStatus{},
	}

	_, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(context.Background(), testdr1,
		metav1.CreateOptions{})
	assert.NoError(t, err)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		_, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(context.Background(), testdr1.Name,
			metav1.GetOptions{})
		if err == nil {
			break
		}
	}

	addLabels := map[string]string{
		"l1": "v1",
		"l2": "v2",
	}

	removeLabels := map[string]bool{
		"l3": true,
		"l4": true,
	}

	_, err = c.updateLabel(context.Background(), testdr1, addLabels, removeLabels)
	assert.NoError(t, err)
	cdr, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(context.Background(), testdr1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	v1, ok := cdr.Labels["l1"]
	assert.True(t, ok)
	assert.True(t, v1 == "v1")

	_, ok = cdr.Labels["l3"]
	assert.False(t, ok)

	assert.Equal(t, len(cdr.Labels), 5)
}

func Test_controller_syncDomainRouteStatus(t *testing.T) {
	c := NewTestController()
	sourceDomain := "alice"
	destDomain := "bob"

	testCdr1 := &kusciaapisv1alpha1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: sourceDomain + "-" + destDomain,
			Labels: map[string]string{
				"l4": "v4",
				"l5": "v5",
				"l1": "v0",
			},
		},
		Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
			DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
				Source:             sourceDomain,
				Destination:        destDomain,
				AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
				TokenConfig: &kusciaapisv1alpha1.TokenConfig{
					TokenGenMethod: kusciaapisv1alpha1.TokenGenMethodRSA,
				},
			},
		},
		Status: kusciaapisv1alpha1.ClusterDomainRouteStatus{},
	}

	testCdr1, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(context.Background(), testCdr1,
		metav1.CreateOptions{})
	assert.NoError(t, err)

	mockToken := kusciaapisv1alpha1.DomainRouteToken{
		IsReady:            true,
		Token:              "xx",
		Revision:           0,
		EffectiveInstances: nil,
	}

	dstDr := &kusciaapisv1alpha1.DomainRoute{
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			IsDestinationAuthorized:  false,
			IsDestinationUnreachable: false,
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				RevisionToken: mockToken,
				Tokens:        []kusciaapisv1alpha1.DomainRouteToken{mockToken},
			},
		},
	}
	srcDr := dstDr.DeepCopy()

	dstDr.Status.TokenStatus.Tokens = []kusciaapisv1alpha1.DomainRouteToken{}
	update, err := c.syncStatusFromDomainroute(testCdr1, srcDr, dstDr)
	assert.NoError(t, err)
	assert.True(t, update)
	cdr, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(c.ctx, testCdr1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.False(t, IsReady(&cdr.Status))

	dstDr.Status.TokenStatus.Tokens = []kusciaapisv1alpha1.DomainRouteToken{mockToken}
	update, err = c.syncStatusFromDomainroute(testCdr1, srcDr, dstDr)
	assert.NoError(t, err)
	assert.True(t, update)
	cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(c.ctx, testCdr1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, IsReady(&cdr.Status))

	srcDr.Status.IsDestinationUnreachable = true
	update, err = c.syncStatusFromDomainroute(testCdr1, srcDr, dstDr)
	assert.NoError(t, err)
	assert.True(t, update)
	cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(c.ctx, testCdr1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.False(t, IsReady(&cdr.Status))

	srcDr.Status.IsDestinationUnreachable = false
	update, err = c.syncStatusFromDomainroute(testCdr1, srcDr, dstDr)
	assert.NoError(t, err)
	assert.True(t, update)
	cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(c.ctx, testCdr1.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, IsReady(&cdr.Status))
}
