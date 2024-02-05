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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestGetKusciaJobCondition(t *testing.T) {
	tests := []struct {
		name         string
		jobStatus    *kusciaapisv1alpha1.KusciaJobStatus
		condType     kusciaapisv1alpha1.KusciaJobConditionType
		generateCond bool
		wantNil      bool
	}{
		{
			name:      "condition doesn't exist and doesn't generate",
			jobStatus: &kusciaapisv1alpha1.KusciaJobStatus{},
			wantNil:   true,
		},
		{
			name:         "condition doesn't exist but generate it",
			jobStatus:    &kusciaapisv1alpha1.KusciaJobStatus{},
			condType:     kusciaapisv1alpha1.JobStartSucceeded,
			generateCond: true,
			wantNil:      false,
		},
		{
			name: "condition exist",
			jobStatus: &kusciaapisv1alpha1.KusciaJobStatus{
				Conditions: []kusciaapisv1alpha1.KusciaJobCondition{
					{
						Type: kusciaapisv1alpha1.JobStartSucceeded,
					},
				},
			},
			condType: kusciaapisv1alpha1.JobStartSucceeded,
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := GetKusciaJobCondition(tt.jobStatus, tt.condType, tt.generateCond)
			assert.Equal(t, got == nil, tt.wantNil)
		})
	}
}

func TestSetKusciaJobCondition(t *testing.T) {
	cond := &kusciaapisv1alpha1.KusciaJobCondition{}
	SetKusciaJobCondition(metav1.Now(), cond, corev1.ConditionFalse, "", "")
	assert.NotEmpty(t, cond.Status)
}
