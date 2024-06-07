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

package handler

import (
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/test/util"
)

func TestNewCreatingHandler(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	deps := &Dependencies{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
		PodLister:    podInformer.Lister(),
		TrLister:     trInformer.Lister(),
	}

	h := NewCreatingHandler(deps)
	if h == nil {
		t.Error("creating handler should not be nil")
	}
}

func TestCreatingHandlerHandle(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg",
	}
	trInformer.Informer().GetStore().Add(tr)

	ns1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
		},
	}
	nsInformer.Informer().GetStore().Add(ns1)

	deps := &Dependencies{
		KubeClient:      kubeFakeClient,
		KusciaClient:    kusciaFakeClient,
		NamespaceLister: nsInformer.Lister(),
		PodLister:       podInformer.Lister(),
		TrLister:        trInformer.Lister(),
	}

	h := NewCreatingHandler(deps)
	tests := []struct {
		name    string
		trg     *kusciaapisv1alpha1.TaskResourceGroup
		wantErr bool
	}{
		{
			name: "task resource group is valid",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg",
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator: "ns1",
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns1",
						},
					},
				},
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseCreating},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := h.Handle(tt.trg)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestCreateTaskResources(t *testing.T) {
	t.Parallel()
	pod := st.MakePod().Namespace("ns1").Name("pod").Obj()
	kubeFakeClient := clientsetfake.NewSimpleClientset(pod)
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	podInformer.Informer().GetStore().Add(pod)

	ns1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
		},
	}
	nsInformer.Informer().GetStore().Add(ns1)

	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg",
	}
	trInformer.Informer().GetStore().Add(tr)

	deps := &Dependencies{
		KubeClient:      kubeFakeClient,
		KusciaClient:    kusciaFakeClient,
		NamespaceLister: nsInformer.Lister(),
		PodLister:       podInformer.Lister(),
		TrLister:        trInformer.Lister(),
	}

	h := NewCreatingHandler(deps)

	tests := []struct {
		name    string
		trg     *kusciaapisv1alpha1.TaskResourceGroup
		wantErr bool
	}{
		{
			name: "task resource does not exist",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg",
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator: "ns1",
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns1",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod",
								},
							},
						},
					},
				},
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseCreating},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.createTaskResources(tt.trg)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestBuildTaskResource(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	containers := []v1.Container{
		{
			Name: "c1",
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Mi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Mi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
		},
	}
	pod1 := st.MakePod().Namespace("ns1").Name("pod1").Containers(containers).Obj()
	podInformer.Informer().GetStore().Add(pod1)

	ns1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
		},
	}
	nsInformer.Informer().GetStore().Add(ns1)

	deps := &Dependencies{
		KubeClient:      kubeFakeClient,
		KusciaClient:    kusciaFakeClient,
		NamespaceLister: nsInformer.Lister(),
		PodLister:       podInformer.Lister(),
		TrLister:        trInformer.Lister(),
	}

	h := NewCreatingHandler(deps)
	if h == nil {
		t.Error("creating handler should not be nil")
	}

	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator: "ns1",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod1",
						},
					},
				},
			},
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg2",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator: "ns1",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod2",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		trg     *kusciaapisv1alpha1.TaskResourceGroup
		wantErr bool
	}{
		{
			name:    "task resource group is not valid",
			trg:     trg2,
			wantErr: true,
		},
		{
			name:    "task resource group is valid",
			trg:     trg1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := h.buildTaskResource(&tt.trg.Spec.Parties[0], tt.trg, nil)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestFindPartyTaskResource(t *testing.T) {
	t.Parallel()
	party := kusciaapisv1alpha1.TaskResourceGroupParty{
		DomainID: "ns1",
		Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
			{
				Name: "pod1",
			},
		},
	}

	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr1",
			Namespace: "ns1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Pods: []kusciaapisv1alpha1.TaskResourcePod{
				{
					Name: "pod1",
				},
			},
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr2",
			Namespace: "ns1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Pods: []kusciaapisv1alpha1.TaskResourcePod{
				{
					Name: "pod2",
				},
			},
		},
	}

	tests := []struct {
		name    string
		party   kusciaapisv1alpha1.TaskResourceGroupParty
		tr      []*kusciaapisv1alpha1.TaskResource
		wantNil bool
	}{
		{
			name:    "does not find party task resource",
			party:   party,
			tr:      []*kusciaapisv1alpha1.TaskResource{tr2},
			wantNil: true,
		},
		{
			name:    "find party task resource",
			party:   party,
			tr:      []*kusciaapisv1alpha1.TaskResource{tr1},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPartyTaskResource(tt.party, tt.tr)
			if got == nil != tt.wantNil {
				t.Errorf("got: %v, want: %v", got == nil, tt.wantNil)
			}
		})
	}
}

func TestGetMinReservedPods(t *testing.T) {
	t.Parallel()
	party1 := &kusciaapisv1alpha1.TaskResourceGroupParty{
		DomainID:        "ns1",
		MinReservedPods: 2,
		Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
			{
				Name: "pod1",
			},
		},
	}

	party2 := &kusciaapisv1alpha1.TaskResourceGroupParty{
		DomainID:        "ns1",
		MinReservedPods: 2,
		Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
			{
				Name: "pod1",
			},
			{
				Name: "pod2",
			},
			{
				Name: "pod3",
			},
		},
	}

	tests := []struct {
		name  string
		party *kusciaapisv1alpha1.TaskResourceGroupParty
		want  int
	}{
		{
			name:  "min reserved pods is equal to 1",
			party: party1,
			want:  1,
		},
		{
			name:  "min reserved pods is equal to 2",
			party: party2,
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMinReservedPods(tt.party, false)
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestGenerateTaskResourceName(t *testing.T) {
	t.Parallel()
	trName := generateTaskResourceName("tr")
	ret := strings.Split(trName, "-")
	if len(ret) != 2 {
		t.Error("result length should be equal to 2")
	}
}
