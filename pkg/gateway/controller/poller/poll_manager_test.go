// Copyright 2024 Ant Group Co., Ltd.
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

package poller

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/tools/cache"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestPollManager_Run(t *testing.T) {
	testServices := makeTestServices()
	kubeClient := kubefake.NewSimpleClientset(testServices...)
	kusciaClient := kusciafake.NewSimpleClientset(makeTestDomainRoutes()...)

	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	svcInformer := kubeInformersFactory.Core().V1().Services()
	drInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()

	pm, err := NewPollManager(true, "alice", "alice-01", svcInformer, drInformer)
	assert.NoError(t, err)
	stopCh := make(chan struct{})
	defer close(stopCh)
	go pm.Run(1, stopCh)
	go kubeInformersFactory.Start(stopCh)
	go kusciaInformerFactory.Start(stopCh)

	cache.WaitForNamedCacheSync("poll manager", stopCh, pm.serviceListerSynced, pm.domainRouteListerSynced)

	pollerExist := func(serviceName, receiverDomain string) bool {
		pm.pollersLock.Lock()
		defer pm.pollersLock.Unlock()
		pollers, ok := pm.pollers[serviceName]
		if !ok {
			return false
		}
		if _, ok := pollers[receiverDomain]; !ok {
			return false
		}
		return true
	}

	t.Run("test service pollers", func(t *testing.T) {
		pollClientExist := false
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if pollerExist("test-service-02", "bob") && pollerExist("test-service-03", "bob") {
				pollClientExist = true
				break
			}
		}
		assert.True(t, pollClientExist)

		assert.NoError(t, kubeClient.CoreV1().Services("alice").Delete(context.Background(), "test-service-03", metav1.DeleteOptions{}))
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if pollerExist("test-service-02", "bob") && !pollerExist("test-service-03", "bob") {
				pollClientExist = false
				break
			}
		}
		assert.False(t, pollClientExist)
	})

	t.Run("default domain route pollers", func(t *testing.T) {
		pollClientExist := false

		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if pollerExist("apiserver", "bob") && pollerExist("kuscia-handshake", "bob") {
				pollClientExist = true
				break
			}
		}
		assert.True(t, pollClientExist)
	})

	t.Run("update domain route", func(t *testing.T) {
		dr, err := kusciaClient.KusciaV1alpha1().DomainRoutes("alice").Get(context.Background(), "carol-alice", metav1.GetOptions{})
		assert.NoError(t, err)
		dr.Spec.Transit = &kusciaapisv1alpha1.Transit{
			TransitMethod: kusciaapisv1alpha1.TransitMethodReverseTunnel,
		}
		_, err = kusciaClient.KusciaV1alpha1().DomainRoutes("alice").Update(context.Background(), dr, metav1.UpdateOptions{})
		assert.NoError(t, err)

		ok := false

		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if pollerExist("apiserver", "carol") && pollerExist("kuscia-handshake", "carol") &&
				pollerExist("test-service-02", "carol") {
				ok = true
				break
			}
		}
		assert.True(t, ok)

		err = kusciaClient.KusciaV1alpha1().DomainRoutes("alice").Delete(context.Background(), "carol-alice", metav1.DeleteOptions{})
		assert.NoError(t, err)

		ok = false
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if !pollerExist("apiserver", "carol") && !pollerExist("kuscia-handshake", "carol") &&
				!pollerExist("test-service-02", "carol") {
				ok = true
				break
			}
		}
		assert.True(t, ok)

		dr, err = kusciaClient.KusciaV1alpha1().DomainRoutes("alice").Get(context.Background(), "bob-alice", metav1.GetOptions{})
		assert.NoError(t, err)
		dr.Spec.Transit = nil
		_, err = kusciaClient.KusciaV1alpha1().DomainRoutes("alice").Update(context.Background(), dr, metav1.UpdateOptions{})
		assert.NoError(t, err)

		ok = false
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if !pollerExist("apiserver", "bob") && !pollerExist("kuscia-handshake", "bob") &&
				!pollerExist("test-service-02", "bob") {
				ok = true
				break
			}
		}
		assert.True(t, ok)
	})

}

func makeTestServices() []runtime.Object {
	return []runtime.Object{
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-01",
				Namespace: "alice",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-02",
				Namespace: "alice",
				Labels: map[string]string{
					common.LabelPortScope: string(kusciaapisv1alpha1.ScopeCluster),
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-03",
				Namespace: "alice",
				Labels: map[string]string{
					common.LabelPortScope: string(kusciaapisv1alpha1.ScopeCluster),
				},
				Annotations: map[string]string{
					common.AccessDomainAnnotationKey: "bob,carol,denis",
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
	}
}

func makeTestDomainRoutes() []runtime.Object {
	return []runtime.Object{
		&kusciaapisv1alpha1.DomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bob-alice",
				Namespace: "alice",
				Labels: map[string]string{
					common.KusciaSourceKey:      "bob",
					common.KusciaDestinationKey: "alice",
				},
			},
			Spec: kusciaapisv1alpha1.DomainRouteSpec{
				Source:      "bob",
				Destination: "alice",
				Transit: &kusciaapisv1alpha1.Transit{
					TransitMethod: kusciaapisv1alpha1.TransitMethodReverseTunnel,
				},
			},
		},
		&kusciaapisv1alpha1.DomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "carol-alice",
				Namespace: "alice",
				Labels: map[string]string{
					common.KusciaSourceKey:      "carol",
					common.KusciaDestinationKey: "alice",
				},
			},
			Spec: kusciaapisv1alpha1.DomainRouteSpec{
				Source:      "carol",
				Destination: "alice",
			},
		},
	}
}
