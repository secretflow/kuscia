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

package nodeutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AddOrUpdateNodeCondition(list []corev1.NodeCondition, newCondition corev1.NodeCondition) ([]corev1.NodeCondition, bool) {
	now := metav1.Now()
	for idx := range list {
		if list[idx].Type == newCondition.Type {
			oldCond := &list[idx]

			changed := oldCond.Status != newCondition.Status ||
				oldCond.Reason != newCondition.Reason || oldCond.Message != newCondition.Message

			oldCond.LastHeartbeatTime = now
			oldCond.Reason = newCondition.Reason
			oldCond.Message = newCondition.Message
			if oldCond.Status != newCondition.Status {
				oldCond.Status = newCondition.Status
				oldCond.LastTransitionTime = now
			}

			return list, changed
		}
	}

	// not found
	newCondition.LastHeartbeatTime = now
	newCondition.LastTransitionTime = now
	return append(list, newCondition), true
}

func IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
