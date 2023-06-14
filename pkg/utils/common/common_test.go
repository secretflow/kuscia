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

package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func TestPatchPodSpec(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
		Spec: corev1.PodSpec{
			NodeName: "test",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset(pod1)
	tests := []struct {
		name    string
		oldObj  *corev1.Pod
		newObj  *corev1.Pod
		wantErr bool
	}{
		{
			name:    "pod spec is not updated",
			oldObj:  pod1,
			newObj:  pod1,
			wantErr: false,
		},
		{
			name:    "pod spec is updated",
			oldObj:  pod1,
			newObj:  pod2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchPod(context.Background(), kubeFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestPatchPodStatus(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
		Status: corev1.PodStatus{
			Phase: "Running",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset(pod1)
	tests := []struct {
		name    string
		oldObj  *corev1.Pod
		newObj  *corev1.Pod
		wantErr bool
	}{
		{
			name:    "pod status is not updated",
			oldObj:  pod1,
			newObj:  pod1,
			wantErr: false,
		},
		{
			name:    "pod status is updated",
			oldObj:  pod1,
			newObj:  pod2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchPodStatus(context.Background(), kubeFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestPatchService(t *testing.T) {
	svc1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc1",
			Namespace: "ns2",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "127.0.0.1",
		},
	}

	svc2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc1",
			Namespace: "ns2",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset(svc1)
	tests := []struct {
		name    string
		oldObj  *corev1.Service
		newObj  *corev1.Service
		wantErr bool
	}{
		{
			name:    "service is not updated",
			oldObj:  svc1,
			newObj:  svc1,
			wantErr: false,
		},
		{
			name:    "service is updated",
			oldObj:  svc1,
			newObj:  svc2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchService(context.Background(), kubeFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestPatchConfigMap(t *testing.T) {
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "ns2",
		},
		Data: map[string]string{"key": "value"},
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "ns2",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset(cm1)
	tests := []struct {
		name    string
		oldObj  *corev1.ConfigMap
		newObj  *corev1.ConfigMap
		wantErr bool
	}{
		{
			name:    "configmap is not updated",
			oldObj:  cm1,
			newObj:  cm2,
			wantErr: false,
		},
		{
			name:    "configmap is updated",
			oldObj:  cm1,
			newObj:  cm2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchConfigMap(context.Background(), kubeFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

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
					Type:   kusciaapisv1alpha1.TaskResourcesGroupCondReserveFailed,
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
			condType: kusciaapisv1alpha1.TaskResourcesGroupCondReserving,
			wantNil:  false,
		},
		{
			name:     "condition exist",
			trg:      trg1,
			condType: kusciaapisv1alpha1.TaskResourcesGroupCondReserveFailed,
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

func TestCompareResourceVersion(t *testing.T) {
	tests := []struct {
		name string
		rv1  string
		rv2  string
		want bool
	}{
		{
			name: "rv1 is greater than rv2",
			rv1:  "2",
			rv2:  "1",
			want: true,
		},
		{
			name: "rv1 is equal to rv2",
			rv1:  "1",
			rv2:  "1",
			want: false,
		},
		{
			name: "rv1 is less than rv2",
			rv1:  "1",
			rv2:  "2",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareResourceVersion(tt.rv1, tt.rv2)
			if got != tt.want {
				t.Errorf(" got %v, want %v", got, tt.want)
			}
		})
	}
}
