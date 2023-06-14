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

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func makeTestDomain(name string) *kusciaapisv1alpha1.Domain {
	count := 1
	return &kusciaapisv1alpha1.Domain{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: name,
		},
		Spec: kusciaapisv1alpha1.DomainSpec{
			Cert: "xxxxxxxx",
			ResourceQuota: &kusciaapisv1alpha1.DomainResourceQuota{
				PodMaxCount: &count,
			},
		},
		Status: &kusciaapisv1alpha1.DomainStatus{
			NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
				{
					Name:    "node-1",
					Version: "v1.0.0",
					Status:  "Ready",
				},
			},
		},
	}
}

func TestGetDomainStatus(t *testing.T) {
	c := &Controller{}
	testCases := []struct {
		name           string
		nodes          []*apicorev1.Node
		expectedStatus *kusciaapisv1alpha1.DomainStatus
	}{
		{
			name:           "want status is nil",
			nodes:          nil,
			expectedStatus: nil,
		},
		{
			name: "want status is not nil",
			nodes: []*apicorev1.Node{
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-1",
					},
					Status: apicorev1.NodeStatus{
						Conditions: []apicorev1.NodeCondition{
							{
								Type:   apicorev1.NodeReady,
								Status: apicorev1.ConditionTrue,
							},
						},
						NodeInfo: apicorev1.NodeSystemInfo{
							KubeletVersion: "v1.0.0",
						},
					},
				},
			},
			expectedStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := c.getDomainStatus(tc.nodes)
			assert.Equal(t, tc.expectedStatus, status)
		})
	}
}

func TestIsDomainStatusEqual(t *testing.T) {
	c := &Controller{}
	testCases := []struct {
		name      string
		oldStatus *kusciaapisv1alpha1.DomainStatus
		newStatus *kusciaapisv1alpha1.DomainStatus
		want      bool
	}{
		{
			name:      "all status are nil",
			oldStatus: nil,
			newStatus: nil,
			want:      true,
		},
		{
			name:      "only old status is nil",
			oldStatus: nil,
			newStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			want: false,
		},
		{
			name: "only new status is nil",
			oldStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			newStatus: nil,
			want:      false,
		},
		{
			name: "all status are not nil and not equal",
			oldStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			newStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-2",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			want: false,
		},
		{
			name: "all status are not nil and equal",
			oldStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			newStatus: &kusciaapisv1alpha1.DomainStatus{
				NodeStatuses: []kusciaapisv1alpha1.NodeStatus{
					{
						Name:    "node-1",
						Version: "v1.0.0",
						Status:  "Ready",
					},
				},
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			equal := c.isDomainStatusEqual(tc.oldStatus, tc.newStatus)
			assert.Equal(t, tc.want, equal)
		})
	}
}

func TestUpdateDomainStatus(t *testing.T) {
	testCases := []struct {
		name    string
		c       *Controller
		domain  *kusciaapisv1alpha1.Domain
		wantErr bool
	}{
		{
			name: "update failure",
			c: &Controller{
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			domain:  makeTestDomain("test"),
			wantErr: true,
		},
		{
			name: "update success",
			c: &Controller{
				kusciaClient: kusciafake.NewSimpleClientset(makeTestDomain("test")),
			},
			domain:  makeTestDomain("test"),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.updateDomainStatus(tc.domain)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
