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

	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestNewHostResourceManager(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opt := &Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
	}

	hrm := NewHostResourcesManager(opt)
	if hrm == nil {
		t.Error("host resources manager should not be nil")
	}
}

func TestHostResourcesManagerStop(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	opt := &Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
	}

	hrm := NewHostResourcesManager(opt)
	hrm.Stop()
	m := hrm.(*hostResourcesManager)
	if m.resourceControllers != nil {
		t.Errorf("host resources manager resourceControllers should be nil")
	}
}

func TestHostResourcesManagerRegisterAndDeregister(t *testing.T) {
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

	opt := &Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
	}

	host, member := "ns1", "ns2"
	hrm := NewHostResourcesManager(opt)
	err := hrm.Register(host, member)
	if err != nil {
		t.Errorf("host resources manager register failed, %v", err.Error())
	}

	m := hrm.(*hostResourcesManager)
	if m.resourceControllers[generateResourceControllerKey(host, member)] == nil {
		t.Errorf("host resources manager resourceControllers should not nil")
	}

	hrm.Deregister(host, member)
	if m.resourceControllers[generateResourceControllerKey(host, member)] != nil {
		t.Errorf("host resources manager resourceControllers should nil")
	}
}

func TestGetHostResourceAccessor(t *testing.T) {
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

	opt := &Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
	}

	host, member := "ns1", "ns2"
	hrm := NewHostResourcesManager(opt)
	hrm.Register(host, member)

	tests := []struct {
		name    string
		host    string
		member  string
		wantNil bool
	}{
		{
			name:    "host resource controller is not exist",
			host:    "ns2",
			member:  "ns3",
			wantNil: true,
		},
		{
			name:    "host resource controller is exist",
			host:    "ns1",
			member:  "ns2",
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hrm.GetHostResourceAccessor(tt.host, tt.member)
			if got == nil != tt.wantNil {
				t.Errorf("get %v, want %v", got != nil, tt.wantNil)
			}
		})
	}
}
