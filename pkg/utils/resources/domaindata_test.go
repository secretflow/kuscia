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

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPatchDomainDataSpec(t *testing.T) {
	dd1 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
		Spec: kusciaapisv1alpha1.DomainDataSpec{
			Name: "test",
		},
	}

	dd2 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(dd1)
	tests := []struct {
		name    string
		oldObj  *kusciaapisv1alpha1.DomainData
		newObj  *kusciaapisv1alpha1.DomainData
		wantErr bool
	}{
		{
			name:    "domaindatagrant spec is not updated",
			oldObj:  dd1,
			newObj:  dd1,
			wantErr: false,
		},
		{
			name:    "domaindatagrant spec is updated",
			oldObj:  dd1,
			newObj:  dd2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchDomainData(context.Background(), kusciaFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestPatchDomainDataStatus(t *testing.T) {
	dd1 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
		Status: kusciaapisv1alpha1.DataStatus{
			Phase: kusciaapisv1alpha1.AvailablePhase,
		},
	}

	dd2 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "ns2",
		},
	}

	kubeFakeClient := kusciaclientsetfake.NewSimpleClientset(dd1)
	tests := []struct {
		name    string
		oldObj  *kusciaapisv1alpha1.DomainData
		newObj  *kusciaapisv1alpha1.DomainData
		wantErr bool
	}{
		{
			name:    "domaindata status is not updated",
			oldObj:  dd1,
			newObj:  dd1,
			wantErr: false,
		},
		{
			name:    "domaindata status is updated",
			oldObj:  dd1,
			newObj:  dd2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PatchDomainDataStatus(context.Background(), kubeFakeClient, tt.oldObj, tt.newObj)
			if got != nil != tt.wantErr {
				t.Errorf(" got %v, want %v", got != nil, tt.wantErr)
			}
		})
	}
}
