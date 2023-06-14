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

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/test/util"
)

func TestNewFailedHandler(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	deps := &Dependencies{
		KusciaClient: kusciaFakeClient,
		TrLister:     trInformer.Lister(),
	}

	h := NewFailedHandler(deps)
	if h == nil {
		t.Error("failed handler should not be nil")
	}
}

func TestFailedHandlerHandle(t *testing.T) {
	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.Labels = map[string]string{
		common.LabelTaskResourceGroup: "trg1",
	}

	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.Labels = map[string]string{
		common.LabelTaskResourceGroup: "trg2",
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)

	deps := &Dependencies{
		KusciaClient: kusciaFakeClient,
		TrLister:     trInformer.Lister(),
	}

	h := NewFailedHandler(deps)

	tests := []struct {
		name    string
		trg     *kusciaapisv1alpha1.TaskResourceGroup
		wantErr bool
	}{
		{
			name: "failed to handle trg1",
			trg: &kusciaapisv1alpha1.TaskResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trg1",
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
			wantErr: true,
		},
		{
			name: "succeed to handle trg2",
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
				Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed},
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
