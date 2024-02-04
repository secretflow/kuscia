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
// WITHOUT WARRANTIES OR CONDITIONS O ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// initialized --> awaitingApproval --> pending --> running --> succeeded
//                                 |--> approvalReject
//  --> Failed
//  --> Cancelled

// InitializedHandler will handle kuscia job in "" or Initialized phase.
type InitializedHandler struct {
	*JobScheduler
}

// NewInitializedHandler return InitializedHandler to handle Initialized kuscia job.
func NewInitializedHandler(deps *Dependencies) *InitializedHandler {
	return &InitializedHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
// It will validate task and do the first scheduling when the job is committed(phase is "" or Initialized).
func (h *InitializedHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.handleInitialized(kusciaJob)
}

// handleInitialized
// Initialized  --> Pending
// Initialized --> AwaitingApproval
func (h *JobScheduler) handleInitialized(job *kusciaapisv1alpha1.KusciaJob) (needUpdateStatus bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	defer updateJobTime(now, job)
	// validate job
	_, ok := h.validateJob(now, job)
	if !ok {
		return true, nil
	}
	// label job with initiator and interConn parties,To notify the interOp controller handle inter connection job.
	hasUpdated, err := h.annotateKusciaJob(job)
	if err != nil {
		return true, err
	}
	if hasUpdated {
		return false, nil
	}
	ownP, _, _ := h.getAllParties(job)
	if job.Status.StageStatus == nil {
		job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
	}
	if job.Status.ApproveStatus == nil {
		job.Status.ApproveStatus = make(map[string]kusciaapisv1alpha1.JobApprovePhase)
	}
	if utilsres.SelfClusterAsInitiator(h.namespaceLister, job.Spec.Initiator, job.Annotations) {
		// self as initiator
		// set own stageStatus to create success
		// set own approvalStatus to approval accepted
		// note: approval accepted status would auto set by program
		for p := range ownP {
			job.Status.StageStatus[p] = kusciaapisv1alpha1.JobCreateStageSucceeded
			job.Status.ApproveStatus[p] = kusciaapisv1alpha1.JobAccepted
		}
	} else {
		// self as partner
		// set own stageStatus to create success
		// set own approvalStatus to awaiting approval
		// note: approval accepted status must be set by user.
		for p := range ownP {
			job.Status.StageStatus[p] = kusciaapisv1alpha1.JobCreateStageSucceeded
		}
	}
	// inter connection job
	if isInterConnJob(job) {
		job.Status.Phase = kusciaapisv1alpha1.KusciaJobAwaitingApproval
		return true, nil
	}
	// inner job
	// set initialized --> pending
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	return true, nil
}
