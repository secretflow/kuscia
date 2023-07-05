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
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// PendingHandler will handle kuscia job in "" or Pending phase.
type PendingHandler struct {
	jobScheduler *JobScheduler
}

// NewPendingHandler return PendingHandler to handle Pending kuscia job.
func NewPendingHandler(deps *Dependencies) *PendingHandler {
	return &PendingHandler{
		jobScheduler: NewJobScheduler(deps.KusciaClient, deps.NamespaceLister, deps.KusciaTaskLister),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
// It will validate task and do the first scheduling when the job is committed(phase is "" or Pending).
func (h *PendingHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.jobScheduler.push(kusciaJob)
}
