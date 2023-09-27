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

package hostresources

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestNewHostResourcesController(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opts := &hostResourcesControllerOptions{
		host:                  "ns1",
		member:                "ns2",
		memberKubeClient:      kubeFakeClient,
		memberKusciaClient:    kusciaFakeClient,
		memberPodLister:       podInformer.Lister(),
		memberServiceLister:   svcInformer.Lister(),
		memberConfigMapLister: cmInformer.Lister(),
		memberTrLister:        trInformer.Lister(),
	}

	_, err := newHostResourcesController(opts)
	if err != nil {
		t.Errorf("new host resources controller failed, %v", err.Error())
	}
}

func TestResourceFilter(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opts := &hostResourcesControllerOptions{
		host:                  "ns1",
		member:                "ns2",
		memberKubeClient:      kubeFakeClient,
		memberKusciaClient:    kusciaFakeClient,
		memberPodLister:       podInformer.Lister(),
		memberServiceLister:   svcInformer.Lister(),
		memberConfigMapLister: cmInformer.Lister(),
		memberTrLister:        trInformer.Lister(),
	}

	c, _ := newHostResourcesController(opts)

	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "object type is invalid",
			obj:  "test",
			want: false,
		},
		{
			name: "pod is invalid",
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns2",
				},
			},
			want: false,
		},
		{
			name: "pod task initiator label is invalid",
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns2",
					Labels: map[string]string{
						common.LabelInitiator:                "ns3",
						kusciaapisv1alpha1.LabelTaskResource: "tr1",
					},
				},
			},
			want: false,
		},
		{
			name: "pod is valid",
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns2",
					Labels: map[string]string{
						common.LabelInitiator:                "ns1",
						kusciaapisv1alpha1.LabelTaskResource: "tr1",
						common.LabelInterConnProtocolType:    string(kusciaapisv1alpha1.InterConnKuscia),
					},
				},
			},
			want: true,
		},
		{
			name: "service is invalid",
			obj: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "ns2",
				},
			},
			want: false,
		},
		{
			name: "service is valid",
			obj: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc",
					Namespace: "ns2",
					Labels: map[string]string{
						common.LabelInitiator:             "ns1",
						common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnKuscia),
					},
				},
			},
			want: true,
		},
		{
			name: "configmap is invalid",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: "ns2",
				},
			},
			want: false,
		},
		{
			name: "configmap is valid",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: "ns2",
					Labels: map[string]string{
						common.LabelInitiator:             "ns1",
						common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnKuscia),
					},
				},
			},
			want: true,
		},
		{
			name: "task resource is invalid",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tr",
					Namespace: "ns2",
				},
			},
			want: false,
		},
		{
			name: "task resource is valid",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tr",
					Namespace: "ns2",
					Labels: map[string]string{
						common.LabelInitiator:             "ns1",
						common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnKuscia),
					},
				},
			},
			want: true,
		},
		{
			name: "object it DeletedFinalStateUnknown type and invalid",
			obj: cache.DeletedFinalStateUnknown{
				Key: "pod",
				Obj: "pod",
			},
			want: false,
		},
		{
			name: "task resource is valid",
			obj: cache.DeletedFinalStateUnknown{
				Key: "ns1/pod",
				Obj: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod",
						Namespace: "ns2",
						Labels: map[string]string{
							common.LabelInitiator:                "ns1",
							kusciaapisv1alpha1.LabelTaskResource: "tr1",
							common.LabelInterConnProtocolType:    string(kusciaapisv1alpha1.InterConnKuscia),
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.resourceFilter(tt.obj)
			if got != tt.want {
				t.Errorf("get %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHostResourceControllerRun(t *testing.T) {
	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()

	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opts := &hostResourcesControllerOptions{
		host:                  "ns1",
		member:                "ns2",
		memberKubeClient:      kubeFakeClient,
		memberKusciaClient:    kusciaFakeClient,
		memberPodLister:       podInformer.Lister(),
		memberServiceLister:   svcInformer.Lister(),
		memberConfigMapLister: cmInformer.Lister(),
		memberTrLister:        trInformer.Lister(),
	}

	c, _ := newHostResourcesController(opts)
	if err := c.run(3); err != nil {
		t.Error("host resource controller run failed")
	}
}

func TestHostResourceControllerStop(t *testing.T) {
	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opts := &hostResourcesControllerOptions{
		host:                  "ns1",
		member:                "ns2",
		memberKubeClient:      kubeFakeClient,
		memberKusciaClient:    kusciaFakeClient,
		memberPodLister:       podInformer.Lister(),
		memberServiceLister:   svcInformer.Lister(),
		memberConfigMapLister: cmInformer.Lister(),
		memberTrLister:        trInformer.Lister(),
	}

	c, _ := newHostResourcesController(opts)
	c.stop()
	_, sd := c.podsQueue.Get()
	if !sd {
		t.Error("host resource controller stop failed")
	}
}

func TestResourcesAccessor(t *testing.T) {
	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opts := &hostResourcesControllerOptions{
		host:                  "ns1",
		member:                "ns2",
		memberKubeClient:      kubeFakeClient,
		memberKusciaClient:    kusciaFakeClient,
		memberPodLister:       podInformer.Lister(),
		memberServiceLister:   svcInformer.Lister(),
		memberConfigMapLister: cmInformer.Lister(),
		memberTrLister:        trInformer.Lister(),
	}

	c, _ := newHostResourcesController(opts)
	kubeClient := c.HostKubeClient()
	if kubeClient == nil {
		t.Error("host kube client should not be nil")
	}

	kusciaClient := c.HostKusciaClient()
	if kusciaClient == nil {
		t.Error("host kuscia client should not be nil")
	}

	podLister := c.HostPodLister()
	if podLister == nil {
		t.Error("host pod lister should not be nil")
	}

	serviceLister := c.HostServiceLister()
	if serviceLister == nil {
		t.Error("host service lister should not be nil")
	}

	cmLister := c.HostConfigMapLister()
	if cmLister == nil {
		t.Error("host configmap lister should not be nil")
	}

	trLister := c.HostTaskResourceLister()
	if trLister == nil {
		t.Error("host task resource lister should not be nil")
	}
}

func TestGetAPIServerURLForHostCluster(t *testing.T) {
	want := "http://apiserver.ns1.svc"
	got := getAPIServerURLForHostCluster("ns1")
	if got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
