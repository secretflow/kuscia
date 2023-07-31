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

package clusterdomainroute

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	dv1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/scheme"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

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

func Test_controller_with_token_rand(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	chStop := make(chan struct{})
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeClient := kubefake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})
	goroutineNumBegin := runtime.NumGoroutine()
	ctx := signals.NewKusciaContextWithStopCh(chStop)
	ic := NewController(ctx, kubeClient, kusciaClient, eventRecorder)
	alice := "alicetoken"
	bob := "bobtoken"
	go func() {
		time.Sleep(300 * time.Millisecond)
		testcdr := &dv1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + bob,
				Labels: map[string]string{
					common.KusciaSourceKey:      alice,
					common.KusciaDestinationKey: bob,
				},
			},
			Spec: dv1.ClusterDomainRouteSpec{
				DomainRouteSpec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRAND,
					},
				},
			},
			Status: dv1.ClusterDomainRouteStatus{
				Conditions: []dv1.ClusterDomainRouteCondition{},
			},
		}
		kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testcdr, metav1.CreateOptions{})
		time.Sleep(100 * time.Millisecond)
		testgateway := &dv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testgw",
				Namespace: bob,
				Labels: map[string]string{
					"auth": "test",
				},
			},
			Status: dv1.GatewayStatus{
				HeartbeatTime: metav1.Time{
					Time: time.Now(),
				},
			},
		}
		_, err := kusciaClient.KusciaV1alpha1().Gateways(bob).Create(ctx, testgateway, metav1.CreateOptions{})
		assert.NoError(t, err)
		testdr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(bob).Get(ctx, testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		testdr.Status.TokenStatus.Tokens[0].EffectiveInstances = []string{"testgw"}
		kusciaClient.KusciaV1alpha1().DomainRoutes(bob).Update(ctx, testdr, metav1.UpdateOptions{})
		time.Sleep(100 * time.Millisecond)
		cdr, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(context.Background(), testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		srcdr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(alice).Get(context.Background(), testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(
			t,
			dv1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testcdr.Name,
					Namespace: alice,
					Labels: map[string]string{
						common.KusciaSourceKey:      alice,
						common.KusciaDestinationKey: bob,
					},
					OwnerReferences: srcdr.OwnerReferences,
				},
				Spec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRAND,
					},
				},
				Status: dv1.DomainRouteStatus{
					TokenStatus: dv1.DomainRouteTokenStatus{
						RevisionToken: cdr.Status.TokenStatus.SourceTokens[0],
						Tokens: []dv1.DomainRouteToken{
							{
								Token:        cdr.Status.TokenStatus.SourceTokens[0].Token,
								Revision:     1,
								RevisionTime: srcdr.Status.TokenStatus.Tokens[0].RevisionTime,
							},
						},
					},
				},
			},
			*srcdr)
		destdr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(bob).Get(context.Background(), testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(
			t,
			dv1.DomainRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testcdr.Name,
					Namespace: bob,
					Labels: map[string]string{
						common.KusciaSourceKey:      alice,
						common.KusciaDestinationKey: bob,
					},
					OwnerReferences: srcdr.OwnerReferences,
				},
				Spec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRAND,
					},
				},
				Status: dv1.DomainRouteStatus{
					TokenStatus: dv1.DomainRouteTokenStatus{
						RevisionToken: dv1.DomainRouteToken{
							Token:        cdr.Status.TokenStatus.DestinationTokens[0].Token,
							Revision:     1,
							RevisionTime: cdr.Status.TokenStatus.DestinationTokens[0].RevisionTime,
						},
						Tokens: []dv1.DomainRouteToken{
							{
								Token:              cdr.Status.TokenStatus.DestinationTokens[0].Token,
								Revision:           1,
								RevisionTime:       destdr.Status.TokenStatus.Tokens[0].RevisionTime,
								EffectiveInstances: []string{"testgw"},
							},
						},
					},
				},
			},
			*destdr)
		err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Delete(ctx, testcdr.Name, metav1.DeleteOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		close(chStop)
	}()
	ic.Run(4)
	ic.Stop()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, goroutineNumBegin, runtime.NumGoroutine())
}

func Test_controller_with_token_rsa(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	chStop := make(chan struct{})
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeClient := kubefake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})
	goroutineNumBegin := runtime.NumGoroutine()
	ctx := signals.NewKusciaContextWithStopCh(chStop)
	ic := NewController(ctx, kubeClient, kusciaClient, eventRecorder)
	alice := "alicersa"
	bob := "bobrsa"
	go func() {
		var err error
		certstr := createCrtString(t)
		aliceDomain := &dv1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice,
			},
			Spec: dv1.DomainSpec{
				Cert: certstr,
			},
		}
		bobDomain := &dv1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: bob,
			},
			Spec: dv1.DomainSpec{
				Cert: certstr,
			},
		}
		_, err = kusciaClient.KusciaV1alpha1().Domains().Create(ctx, aliceDomain, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().Domains().Create(ctx, bobDomain, metav1.CreateOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		alicegateway := &dv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testgw" + alice,
				Namespace: alice,
				Labels: map[string]string{
					"auth": "test",
				},
			},
			Status: dv1.GatewayStatus{
				HeartbeatTime: metav1.Time{
					Time: time.Now(),
				},
			},
		}
		bobgateway := &dv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testgw" + bob,
				Namespace: bob,
				Labels: map[string]string{
					"auth": "test",
				},
			},
			Status: dv1.GatewayStatus{
				HeartbeatTime: metav1.Time{
					Time: time.Now(),
				},
			},
		}
		_, err = kusciaClient.KusciaV1alpha1().Gateways(alice).Create(ctx, alicegateway, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().Gateways(bob).Create(ctx, bobgateway, metav1.CreateOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		testcdr := &dv1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + bob,
				Labels: map[string]string{
					common.KusciaSourceKey:      alice,
					common.KusciaDestinationKey: bob,
				},
			},
			Spec: dv1.ClusterDomainRouteSpec{
				DomainRouteSpec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRSA,
					},
				},
			},
			Status: dv1.ClusterDomainRouteStatus{
				Conditions: []dv1.ClusterDomainRouteCondition{},
			},
		}
		_, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testcdr, metav1.CreateOptions{})
		assert.NoError(t, err)
		nlog.Debug("create ", testcdr.Name)
		time.Sleep(100 * time.Millisecond)

		srcdr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(alice).Get(ctx, testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		srcdr.Status.TokenStatus.RevisionToken.Token = "alicetestToken"
		srcdr.Status.TokenStatus.RevisionToken.EffectiveInstances = []string{alicegateway.Name}
		_, err = kusciaClient.KusciaV1alpha1().DomainRoutes(alice).Update(ctx, srcdr, metav1.UpdateOptions{})
		assert.NoError(t, err)

		destdr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(bob).Get(ctx, testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		destdr.Status.TokenStatus.RevisionToken.Token = "bobtestToken"
		destdr.Status.TokenStatus.RevisionToken.EffectiveInstances = []string{bobgateway.Name}
		_, err = kusciaClient.KusciaV1alpha1().DomainRoutes(bob).Update(ctx, destdr, metav1.UpdateOptions{})
		assert.NoError(t, err)
		nlog.Debug("update " + alice + "-" + bob)
		time.Sleep(100 * time.Millisecond)

		cdr, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(context.Background(), testcdr.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), cdr.Status.TokenStatus.Revision)
		assert.Equal(t, srcdr.Status.TokenStatus.RevisionToken.Token, cdr.Status.TokenStatus.SourceTokens[0].Token)
		assert.Equal(t, destdr.Status.TokenStatus.RevisionToken.Token, cdr.Status.TokenStatus.DestinationTokens[0].Token)
		time.Sleep(100 * time.Millisecond)
		close(chStop)
	}()
	ic.Run(4)
	ic.Stop()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, goroutineNumBegin, runtime.NumGoroutine())
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
	goroutineNumBegin := runtime.NumGoroutine()
	ctx := signals.NewKusciaContextWithStopCh(chStop)
	ic := NewController(ctx, kubeClient, kusciaClient, eventRecorder)
	alice := "aliceaddlabel"
	bob := "bobaddlabel"
	charlie := "charlieaddlabel"
	go func() {
		time.Sleep(300 * time.Millisecond)
		testcdr := &dv1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + bob,
			},
			Spec: dv1.ClusterDomainRouteSpec{
				DomainRouteSpec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        bob,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRAND,
					},
				},
			},
			Status: dv1.ClusterDomainRouteStatus{
				Conditions: []dv1.ClusterDomainRouteCondition{},
			},
		}
		testcdr2 := &dv1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: alice + "-" + charlie,
				Labels: map[string]string{
					"auth": "test",
				},
			},
			Spec: dv1.ClusterDomainRouteSpec{
				DomainRouteSpec: dv1.DomainRouteSpec{
					Source:             alice,
					Destination:        charlie,
					AuthenticationType: dv1.DomainAuthenticationToken,
					TokenConfig: &dv1.TokenConfig{
						TokenGenMethod: dv1.TokenGenMethodRAND,
					},
				},
			},
			Status: dv1.ClusterDomainRouteStatus{
				Conditions: []dv1.ClusterDomainRouteCondition{},
			},
		}
		nlog.Debug("create ", testcdr.Name, " ", testcdr2.Name)
		_, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testcdr, metav1.CreateOptions{})
		assert.NoError(t, err)
		_, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testcdr2, metav1.CreateOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		close(chStop)
	}()
	ic.Run(4)
	ic.Stop()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, goroutineNumBegin, runtime.NumGoroutine())
	cdr, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(ctx, alice+"-"+bob, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, alice, cdr.Labels[common.KusciaSourceKey])
	assert.Equal(t, bob, cdr.Labels[common.KusciaDestinationKey])
	cdr2, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(ctx, alice+"-"+charlie, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, alice, cdr2.Labels[common.KusciaSourceKey])
	assert.Equal(t, charlie, cdr2.Labels[common.KusciaDestinationKey])
}

func Test_doValidate(t *testing.T) {
	c := &Controller{}
	alice := "alicedovallidate"
	bob := "bobdovalidate"
	testcdr := &dv1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: alice + "-" + bob,
			Labels: map[string]string{
				"auth": "test",
			},
		},
		Spec: dv1.ClusterDomainRouteSpec{
			DomainRouteSpec: dv1.DomainRouteSpec{
				AuthenticationType: dv1.DomainAuthenticationToken,
			},
		},
		Status: dv1.ClusterDomainRouteStatus{
			Conditions: []dv1.ClusterDomainRouteCondition{},
		},
	}
	assert.Equal(t, fmt.Sprintf("%s source or destination is null", testcdr.Name), c.doValidate(context.Background(), testcdr).Error())

	testcdr.Spec.Source = alice
	testcdr.Spec.Destination = bob
	assert.Equal(t, fmt.Sprintf("clusterdomainroute %s Spec.Authentication.Token is null", testcdr.Name), c.doValidate(context.Background(), testcdr).Error())

	testcdr.Spec.AuthenticationType = dv1.DomainAuthenticationMTLS
	assert.Equal(t, fmt.Sprintf("clusterdomainroute %s Spec.MTLSConfig is null", testcdr.Name), c.doValidate(context.Background(), testcdr).Error())

	testcdr.Spec.MTLSConfig = &dv1.DomainRouteMTLSConfig{
		SourceClientCert: "",
	}
	assert.Equal(t, fmt.Sprintf("clusterdomainroute %s Spec.MTLSConfig.SourceClientCert is null", testcdr.Name), c.doValidate(context.Background(), testcdr).Error())

	testcdr.Spec.MTLSConfig.SourceClientCert = createCrtString(t)
	assert.NoError(t, c.doValidate(context.Background(), testcdr))

	testcdr.Spec.BodyEncryption = &dv1.BodyEncryption{
		Algorithm: dv1.BodyEncryptionAlgorithmAES,
	}
	assert.Equal(t, fmt.Sprintf("clusterdomainroute %s Spec.Authentication.Token is null", testcdr.Name), c.doValidate(context.Background(), testcdr).Error())

	testcdr.Spec.TokenConfig = &dv1.TokenConfig{
		TokenGenMethod: dv1.TokenGenMethodRAND,
	}
	assert.NoError(t, c.doValidate(context.Background(), testcdr))
}

func Test_doValidate_NoDomain(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	alice := "aliceNodomain"
	bob := "bobNodomain"
	testcdr := &dv1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: alice + "-" + bob,
			Labels: map[string]string{
				"auth": "test",
			},
		},
		Spec: dv1.ClusterDomainRouteSpec{
			DomainRouteSpec: dv1.DomainRouteSpec{
				Source:             alice,
				Destination:        bob,
				AuthenticationType: dv1.DomainAuthenticationToken,
				TokenConfig: &dv1.TokenConfig{
					TokenGenMethod: dv1.TokenGenMethodRSA,
				},
			},
		},
		Status: dv1.ClusterDomainRouteStatus{
			Conditions: []dv1.ClusterDomainRouteCondition{},
		},
	}

	chStop := make(chan struct{})
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeClient := kubefake.NewSimpleClientset()
	ctx := signals.NewKusciaContextWithStopCh(chStop)
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, time.Second*30)
	clusterDomainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().ClusterDomainRoutes()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})
	c := &Controller{
		kusciaClient:                   kusciaClient,
		domainLister:                   domainInformer.Lister(),
		domainListerSynced:             domainInformer.Informer().HasSynced,
		clusterDomainRouteLister:       clusterDomainRouteInformer.Lister(),
		clusterDomainRouteListerSynced: clusterDomainRouteInformer.Informer().HasSynced,
		recorder:                       eventRecorder,
		workqueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ClusterDomainRoutes"),
	}
	domainInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, newOne interface{}) {
				newNS, ok := newOne.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				c.syncClusterDomainRoute(newNS.Name)
			},
		},
	)
	kusciaInformerFactory.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), c.domainListerSynced, c.clusterDomainRouteListerSynced)
	assert.Equal(t, alice+`-`+bob+`'s source or destination public key is nil, try to sync from domain cert and wait retry`, c.doValidate(context.Background(), testcdr).Error())
	kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, testcdr, metav1.CreateOptions{})
	domain := &dv1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: alice,
		},
		Spec: dv1.DomainSpec{
			Cert: "",
		},
	}
	kusciaClient.KusciaV1alpha1().Domains().Create(ctx, domain, metav1.CreateOptions{})
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, alice+`-`+bob+`'s source or destination public key is nil, try to sync from domain cert and wait retry`, c.doValidate(ctx, testcdr).Error())
	domain.Spec.Cert = createCrtString(t)
	kusciaClient.KusciaV1alpha1().Domains().Update(ctx, domain, metav1.UpdateOptions{})
	domain.Name = bob
	kusciaClient.KusciaV1alpha1().Domains().Create(ctx, domain, metav1.CreateOptions{})
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, alice+`-`+bob+`'s source or destination public key is nil, try to sync from domain cert and wait retry`, c.doValidate(ctx, testcdr).Error())
}

func Test_Name(t *testing.T) {
	c := Controller{}
	assert.Equal(t, controllerName, c.Name())
}
