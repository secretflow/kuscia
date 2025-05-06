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

func TestNewTaskResourceGroupPhaseHandlerFactory(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	deps := &Dependencies{
		KubeClient:      kubeFakeClient,
		KusciaClient:    kusciaFakeClient,
		NamespaceLister: nsInformer.Lister(),
		PodLister:       podInformer.Lister(),
		TrLister:        trInformer.Lister(),
	}

	f := NewTaskResourceGroupPhaseHandlerFactory(deps)
	if f == nil {
		t.Error("task resource group phase handler factory should not be nil")
	}

	h := f.GetTaskResourceGroupPhaseHandler(kusciaapisv1alpha1.TaskResourceGroupPhasePending)
	if h == nil {
		t.Error("pending phase handler should not be nil")
	}
}

func TestUpdatePodAnnotations(t *testing.T) {
	t.Parallel()
	pod1 := st.MakePod().Namespace("ns1").Name("pod1").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg1").
		Label(common.LabelTaskResourceGroupUID, "111").Obj()
	pod2 := st.MakePod().Namespace("ns1").Name("pod2").
		Annotation(common.TaskResourceGroupAnnotationKey, "trg2").
		Label(common.LabelTaskResourceGroupUID, "222").Obj()
	kubeFakeClient := clientsetfake.NewSimpleClientset(pod1)
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	podInformer.Informer().GetStore().Add(pod1)
	podInformer.Informer().GetStore().Add(pod2)

	tests := []struct {
		name    string
		trgUID  string
		wantErr bool
	}{
		{
			name:    "failed to patch pod",
			trgUID:  "222",
			wantErr: true,
		},
		{
			name:    "succeed to patch pod",
			trgUID:  "111",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updatePodAnnotations(tt.trgUID, podInformer.Lister(), kubeFakeClient)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestPatchTaskResourceStatus(t *testing.T) {
	t.Parallel()
	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg1",
	}
	tr1.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "111",
	}

	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg2",
	}
	tr2.Labels = map[string]string{
		common.LabelTaskResourceGroupUID: "111",
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)

	tests := []struct {
		name       string
		trg        *kusciaapisv1alpha1.TaskResourceGroup
		trPhase    kusciaapisv1alpha1.TaskResourcePhase
		trCondType kusciaapisv1alpha1.TaskResourceConditionType
		wantErr    bool
	}{
		{
			name: "failed to patch task resource",
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
						},
					},
				},
			},
			trPhase:    kusciaapisv1alpha1.TaskResourcePhasePending,
			trCondType: kusciaapisv1alpha1.TaskResourceCondPending,
			wantErr:    true,
		},
		{
			name: "succeed to patch task resource",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg2",
				},
				Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
					Initiator: "ns1",
					Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
						{
							DomainID: "ns1",
						},
					},
				},
			},
			trPhase:    kusciaapisv1alpha1.TaskResourcePhasePending,
			trCondType: kusciaapisv1alpha1.TaskResourceCondPending,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := patchTaskResourceStatus(tt.trg, tt.trPhase, tt.trCondType, "", kusciaFakeClient, trInformer.Lister())
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}
