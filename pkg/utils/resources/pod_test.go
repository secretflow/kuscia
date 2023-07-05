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
	clientsetfake "k8s.io/client-go/kubernetes/fake"
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
