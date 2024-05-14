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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// FailedHandler will handle kuscia job in Failed phase.
type FailedHandler struct {
	*JobScheduler
}

// NewFailedHandler return FailedHandler to handle Failed kuscia job.
func NewFailedHandler(deps *Dependencies) *FailedHandler {
	return &FailedHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
// It will do some tail-in work when the job phase is failed.
func (h *FailedHandler) HandlePhase(job *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	// handle stage command, check if the stage command matches the phase of job
	if hasReconciled, err := h.handleStageCommand(now, job); err != nil || hasReconciled {
		return hasReconciled, err
	}

	if job.Status.CompletionTime != nil {
		return false, nil
	}

	// stop the running task
	if err = h.stopTasks(now, job); err != nil {
		nlog.Errorf("Stop 'runnning' task of job: %s failed, error: %s.", job.Name, err.Error())
		return false, err
	}

	setRunningTaskStatusToFailed(&job.Status)
	job.Status.CompletionTime = &now
	return true, nil
}
