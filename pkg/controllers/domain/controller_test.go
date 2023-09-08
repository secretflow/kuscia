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

package domain

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

func TestNewController(t *testing.T) {
	stopCh := make(chan struct{})
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, apicorev1.EventSource{Component: "domain-controller"})

	c := NewController(signals.NewKusciaContextWithStopCh(stopCh), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})
	go func() {
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()
	err := c.Run(1)
	assert.NoError(t, err)
}

func TestMatchLabels(t *testing.T) {
	c := Controller{}
	testCases := []struct {
		name string
		obj  apismetav1.Object
		want bool
	}{
		{
			name: "label match",
			obj: &apicorev1.Namespace{
				ObjectMeta: apismetav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{common.LabelDomainName: "test"},
				},
			},
			want: true,
		},
		{
			name: "label doesn't match",
			obj: &apicorev1.Namespace{
				ObjectMeta: apismetav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"test": "test"},
				},
			},
			want: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := c.matchLabels(tc.obj)
			assert.Equal(t, tc.want, ok)
		})
	}
}

func TestEnqueueDomain(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	domain := makeTestDomain("ns1")
	cc.enqueueDomain(domain)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestEnqueueResourceQuota(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	rq := makeTestResourceQuota("ns1")
	cc.enqueueDomain(rq)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestEnqueueNamespace(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	rq := makeTestNamespace("ns1")
	cc.enqueueDomain(rq)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestStop(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	cc.Stop()
	_, shutdown := cc.workqueue.Get()
	assert.Equal(t, true, shutdown)
}

func TestSyncHandler(t *testing.T) {
	domain1 := makeTestDomain("ns1")
	domain2 := makeTestDomain("ns2")
	ns1 := makeTestNamespace("ns1")
	ns3 := makeTestNamespace("ns3")

	kusciaFakeClient := kusciafake.NewSimpleClientset(domain1, domain2)
	kubeFakeClient := kubefake.NewSimpleClientset(ns1)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)

	dmInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns3)

	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: record.NewFakeRecorder(2),
	})
	cc := c.(*Controller)
	cc.domainLister = dmInformer.Lister()
	cc.namespaceLister = nsInformer.Lister()

	testCases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "domain is not found",
			key:     "ns3",
			wantErr: false,
		},
		{
			name:    "create domain namespace",
			key:     "ns2",
			wantErr: false,
		},
		{
			name:    "update domain namespace",
			key:     "ns1",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.syncHandler(context.Background(), tc.key)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestName(t *testing.T) {
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	kubeFakeClient := kubefake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	got := c.Name()
	assert.Equal(t, controllerName, got)
}
