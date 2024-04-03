// Copyright 2024 Ant Group Co., Ltd.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// SuspendedHandler will handle kuscia job in suspend phase.
type SuspendedHandler struct {
	*JobScheduler
}

// NewSuspendedHandler return SuspendedHandler to handle 'suspend' kuscia job.
func NewSuspendedHandler(deps *Dependencies) *SuspendedHandler {
	return &SuspendedHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
func (h *SuspendedHandler) HandlePhase(job *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	// handle stage command, check if the stage command matches the phase of job
	if hasReconciled, err := h.handleStageCommand(now, job); err != nil || hasReconciled {
		return true, err
	}

	for taskID, phase := range job.Status.TaskStatus {
		if phase != kusciaapisv1alpha1.TaskFailed && phase != kusciaapisv1alpha1.TaskSucceeded {
			job.Status.TaskStatus[taskID] = kusciaapisv1alpha1.TaskFailed
			needUpdate = true
		}
	}
	return needUpdate, nil
}
