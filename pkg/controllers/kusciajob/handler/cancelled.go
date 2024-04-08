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
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// CancelledHandler will handle kuscia job in Cancelled phase.
type CancelledHandler struct {
	*JobScheduler
}

// NewCancelledHandler return CancelledHandler to handle Cancelled kuscia job.
func NewCancelledHandler(deps *Dependencies) *CancelledHandler {
	return &CancelledHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
func (h *CancelledHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.handleCancelled(kusciaJob)
}

// handleCancelled
// Cancelled is completed phase
func (h *CancelledHandler) handleCancelled(job *kusciaapisv1alpha1.KusciaJob) (needUpdateStatus bool, err error) {
	// do nothing
	return
}
