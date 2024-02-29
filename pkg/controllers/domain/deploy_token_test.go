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

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestUpdateDomainToken(t *testing.T) {
	c := Controller{}
	testCases := []struct {
		name         string
		domain       *kusciaapisv1alpha1.Domain
		shouldUpdate bool
		size         int
	}{
		{
			name:         "nilDomainStatus",
			domain:       &kusciaapisv1alpha1.Domain{},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "emptyDomainStatus",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "oneUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              common.DeployTokenUnusedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: false,
			size:         unusedLimit,
		},
		{
			name: "emptyToken",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              "",
							State:              common.DeployTokenUnusedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "unsupportedState",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              "bad",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "oneUsed",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              common.DeployTokenUsedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         usedLimit + unusedLimit,
		},
		{
			name: "oneUsedAndOneUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              common.DeployTokenUnusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              common.DeployTokenUsedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			size:         usedLimit + unusedLimit,
			shouldUpdate: false,
		},
		{
			name: "twoUsedAndTwoUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              common.DeployTokenUnusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              common.DeployTokenUnusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              common.DeployTokenUsedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              common.DeployTokenUsedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         usedLimit + unusedLimit,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			domain := testCase.domain
			var oldTokenStatuses []kusciaapisv1alpha1.DeployTokenStatus
			if domain.Status != nil {
				oldTokenStatuses = domain.Status.DeployTokenStatuses
			}
			newTokenStatuses := c.newDomainTokenStatus(domain)
			assert.Equal(t, !c.isTokenStatusEqual(oldTokenStatuses, newTokenStatuses), testCase.shouldUpdate)
			assert.Equal(t, len(newTokenStatuses), testCase.size, testCase.name)
		})
	}
}
