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

func TestPatchTaskResource(t *testing.T) {
	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr1",
			Namespace: "ns2",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			MinReservedPods:         2,
			ResourceReservedSeconds: 15,
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr1",
			Namespace: "ns2",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			MinReservedPods: 2,
		},
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr1)
	tests := []struct {
		name    string
		oldObj  *kusciaapisv1alpha1.TaskResource
		newObj  *kusciaapisv1alpha1.TaskResource
		wantErr bool
	}{
		{
			name:    "task resource is not updated",
			oldObj:  tr1,
			newObj:  tr1,
			wantErr: false,
		},
		{
			name:    "task resource is updated",
			oldObj:  tr1,
			newObj:  tr2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchTaskResource(context.Background(), kusciaFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestGetTaskResourceCondition(t *testing.T) {
	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr1",
			Namespace: "ns1",
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase: kusciaapisv1alpha1.TaskResourcePhaseFailed,
			Conditions: []kusciaapisv1alpha1.TaskResourceCondition{
				{
					Status: corev1.ConditionTrue,
					Reason: "failure",
					Type:   kusciaapisv1alpha1.TaskResourceCondFailed,
				},
			},
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr2",
			Namespace: "ns1",
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase: kusciaapisv1alpha1.TaskResourcePhaseReserving,
		},
	}

	tests := []struct {
		name     string
		tr       *kusciaapisv1alpha1.TaskResource
		condType kusciaapisv1alpha1.TaskResourceConditionType
		wantNil  bool
	}{
		{
			name:     "condition does not exist",
			tr:       tr2,
			condType: kusciaapisv1alpha1.TaskResourceCondReserving,
			wantNil:  false,
		},
		{
			name:     "condition exist",
			tr:       tr1,
			condType: kusciaapisv1alpha1.TaskResourceCondFailed,
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTaskResourceCondition(&tt.tr.Status, tt.condType)
			if got == nil != tt.wantNil {
				t.Errorf(" got %v, want %v", got != nil, tt.wantNil)
			}
		})
	}
}
