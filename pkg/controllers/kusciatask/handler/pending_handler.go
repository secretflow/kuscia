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

package handler

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// PendingHandler is used to handle kuscia task which phase is pending.
type PendingHandler struct {
}

// NewPendingHandler returns a PendingHandler instance.
func NewPendingHandler() *PendingHandler {
	return &PendingHandler{}
}

// Handle is used to perform the real logic.
func (h *PendingHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	now := metav1.Now().Rfc3339Copy()
	message := "KusciaTask initialized successfully"
	kusciaTask.Status.StartTime = &now
	kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskCreating
	kusciaTask.Status.Reason = "KusciaTaskInitialized"
	kusciaTask.Status.Message = message
	kusciaTask.Status.Conditions = []kusciaapisv1alpha1.KusciaTaskCondition{
		{
			Type:               kusciaapisv1alpha1.TaskInitialized,
			Status:             v1.ConditionTrue,
			LastTransitionTime: now,
		},
	}

	return true, nil
}
