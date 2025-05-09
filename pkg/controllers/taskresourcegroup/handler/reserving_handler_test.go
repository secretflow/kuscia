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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/tests/util"
)

func TestNewReservingHandler(t *testing.T) {
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

	h := NewReservingHandler(deps)
	if h == nil {
		t.Error("reserving handler should not be nil")
	}
}

func TestReservingHandlerHandle(t *testing.T) {
	t.Parallel()
	pod1 := st.MakePod().Namespace("ns1").Name("pod1").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg1").
		Label(common.LabelTaskResourceGroupUID, "111").Obj()
	pod2 := st.MakePod().Namespace("ns2").Name("pod2").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg2").
		Label(common.LabelTaskResourceGroupUID, "222").Obj()
	pod3 := st.MakePod().Namespace("ns3").Name("pod3").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg2").
		Label(common.LabelTaskResourceGroupUID, "222").Obj()
	pod4 := st.MakePod().Namespace("ns3").Name("pod4").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg2").
		Label(common.LabelTaskResourceGroupUID, "222").Obj()

	tr1 := util.MakeTaskResource("ns1", "tr1", 1, nil)
	tr1.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg1",
	}
	tr1.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "111",
	}

	tr2 := util.MakeTaskResource("ns2", "tr2", 1, nil)
	tr2.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg2",
	}
	tr2.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "222",
	}
	tr2.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserved

	tr3 := util.MakeTaskResource("ns3", "tr3", 1, nil)
	tr3.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg2",
	}
	tr3.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "222",
	}
	tr3.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserved

	tr4 := util.MakeTaskResource("ns3", "tr4", 1, nil)
	tr4.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg2",
	}
	tr4.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "222",
	}
	tr4.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseFailed

	kubeFakeClient := clientsetfake.NewSimpleClientset(pod2, pod3, pod4)
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr1, tr2, tr3, tr4)
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	podInformer.Informer().GetStore().Add(pod1)
	podInformer.Informer().GetStore().Add(pod2)
	podInformer.Informer().GetStore().Add(pod3)
	podInformer.Informer().GetStore().Add(pod4)

	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)
	trInformer.Informer().GetStore().Add(tr3)
	trInformer.Informer().GetStore().Add(tr4)

	deps := &Dependencies{
		KubeClient:      kubeFakeClient,
		KusciaClient:    kusciaFakeClient,
		PodLister:       podInformer.Lister(),
		TrLister:        trInformer.Lister(),
		NamespaceLister: nsInformer.Lister(),
	}

	h := NewReservingHandler(deps)

	tests := []struct {
		name     string
		trg      *kusciaapisv1alpha1.TaskResourceGroup
		tr2Phase kusciaapisv1alpha1.TaskResourcePhase
		tr3Phase kusciaapisv1alpha1.TaskResourcePhase
		want     kusciaapisv1alpha1.TaskResourceGroupPhase
	}{
		{
			name: "update pod annotation failed",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg1",
					UID:  types.UID("111"),
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
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving,
		},
		{
			name: "min reserved member is invalid",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg2",
					UID:  types.UID("222"),
					Annotations: map[string]string{
						common.SelfClusterAsInitiatorAnnotationKey: "true",
					},
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator:          "ns2",
					MinReservedMembers: 10,
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns2",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod2",
								},
							},
						},
						{
							DomainID: "ns3",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod3",
								},
							},
						},
					},
				},
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
		},
		{
			name: "reserved count is greater than min reserved members",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg2",
					UID:  types.UID("222"),
					Annotations: map[string]string{
						common.SelfClusterAsInitiatorAnnotationKey: "true",
					},
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator:          "ns2",
					MinReservedMembers: 2,
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns2",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod2",
								},
							},
						},
						{
							DomainID: "ns3",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod3",
								},
							},
						},
						{
							DomainID: "ns3",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod4",
								},
							},
						},
					},
				},
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseReserved,
		},
		{
			name: "reserved count is not greater than min reserved members",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg2",
					UID:  types.UID("222"),
					Annotations: map[string]string{
						common.SelfClusterAsInitiatorAnnotationKey: "true",
					},
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator:          "ns2",
					MinReservedMembers: 3,
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns2",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod2",
								},
							},
						},
						{
							DomainID: "ns3",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod3",
								},
							},
						},
						{
							DomainID: "ns3",
							Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
								{
									Name: "pod4",
								},
							},
						},
					},
				},
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving},
			},
			want: kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed,
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
