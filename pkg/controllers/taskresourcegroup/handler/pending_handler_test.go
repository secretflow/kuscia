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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestNewPendingHandler(t *testing.T) {
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

	h := NewPendingHandler(deps)
	if h == nil {
		t.Error("pending handler should not be nil")
	}
}

func TestPendingHandlerHandle(t *testing.T) {
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

	h := NewPendingHandler(deps)

	tests := []struct {
		name string
		trg  *kusciaapisv1alpha1.TaskResourceGroup
		want kusciaapisv1alpha1.TaskResourceGroupPhase
	}{
		{
			name: "trg initiator is invalid",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg1",
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator: "alice",
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns1",
						},
					},
				},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
		},
		{
			name: "handle trg2 successfully",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg2",
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator: "ns1",
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID:         "ns1",
							MinReservedPods:  0,
							TaskResourceName: "tr2",
						},
					},
				},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseCreating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.Handle(tt.trg)
			got := tt.trg.Status.Phase
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}
