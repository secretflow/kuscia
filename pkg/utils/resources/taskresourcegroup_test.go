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

package resources

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func TestPatchTaskResourceGroupStatus(t *testing.T) {
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserved,
		},
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(trg1)
	tests := []struct {
		name    string
		oldObj  *kusciaapisv1alpha1.TaskResourceGroup
		newObj  *kusciaapisv1alpha1.TaskResourceGroup
		wantErr bool
	}{
		{
			name:    "task resource group status is not updated",
			oldObj:  trg1,
			newObj:  trg1,
			wantErr: false,
		},
		{
			name:    "task resource group status is updated",
			oldObj:  trg1,
			newObj:  trg2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchTaskResourceGroupStatus(context.Background(), kusciaFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestGetTaskResourceGroupCondition(t *testing.T) {
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
			Conditions: []kusciaapisv1alpha1.TaskResourceGroupCondition{
				{
					Status: corev1.ConditionTrue,
					Type:   kusciaapisv1alpha1.TaskResourceGroupFailed,
				},
			},
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg2",
		},

		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving,
		},
	}

	tests := []struct {
		name     string
		trg      *kusciaapisv1alpha1.TaskResourceGroup
		condType kusciaapisv1alpha1.TaskResourceGroupConditionType
		wantNil  bool
	}{
		{
			name:     "condition does not exist",
			trg:      trg2,
			condType: kusciaapisv1alpha1.TaskResourcesListed,
			wantNil:  false,
		},
		{
			name:     "condition exist",
			trg:      trg1,
			condType: kusciaapisv1alpha1.TaskResourceGroupFailed,
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := GetTaskResourceGroupCondition(&tt.trg.Status, tt.condType)
			if got == nil != tt.wantNil {
				t.Errorf(" got %v, want %v", got != nil, tt.wantNil)
			}
		})
	}
}
