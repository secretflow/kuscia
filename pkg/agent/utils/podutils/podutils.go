// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podutils

import (
	v1 "k8s.io/api/core/v1"
)

func UpdateCondition(status *v1.PodStatus, conditionType v1.PodConditionType, condition v1.PodCondition) {
	conditionIndex := -1
	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			conditionIndex = i
			break
		}
	}
	if conditionIndex != -1 {
		status.Conditions[conditionIndex] = condition
	} else {
		status.Conditions = append(status.Conditions, condition)
	}
}
