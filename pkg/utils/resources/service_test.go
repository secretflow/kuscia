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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
)

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

func TestGenerateServiceName(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		portName string
		expected string
	}{
		{
			name:     "prefix-port is less than 63 characters",
			prefix:   "service-test-11111111",
			portName: "domain",
			expected: "service-test-11111111-domain",
		},
		{
			name:     "prefix-port is equal to 63 characters",
			prefix:   "abc-012345678-012345678-012345678-012345678-012345678-01",
			portName: "domain",
			expected: "abc-012345678-012345678-012345678-012345678-012345678-01-domain",
		},
		{
			name:     "prefix is greater than 63 characters",
			prefix:   "abc-012345678-012345678-012345678-012345678-012345678-012",
			portName: "domain",
			expected: "svc-abc-012345678-012345678-012345678-0-domain",
		},
		{
			name:     "prefix is digital",
			prefix:   "123-456-789",
			portName: "domain",
			expected: "svc-123-456-789-domain",
		},
		{
			name:     "prefix is digital and svc-prefix-port length greater than 63",
			prefix:   "12-012345678-012345678-012345678-012345678-012345678",
			portName: "domain",
			expected: "svc-12-012345678-012345678-012345678-012345678-012345678-domain",
		},
		{
			name:     "prefix is digital and svc-prefix-port length greater than 63",
			prefix:   "123-012345678-012345678-012345678-012345678-012345678",
			portName: "domain",
			expected: "svc-123-012345678-012345678-012345678-0-domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateServiceName(tt.prefix, tt.portName)
			assert.True(t, strings.HasPrefix(got, tt.expected))
		})
	}
}
