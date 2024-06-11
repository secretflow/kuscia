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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
)

// AwaitingApprovalHandler will handle kuscia job in AwaitingApproval phase.
type AwaitingApprovalHandler struct {
	*JobScheduler
}

// NewAwaitingApprovalHandler return AwaitingApprovalHandler to handle AwaitingApproval kuscia job.
func NewAwaitingApprovalHandler(deps *Dependencies) *AwaitingApprovalHandler {
	return &AwaitingApprovalHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
func (h *AwaitingApprovalHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.handleAwaitingApproval(kusciaJob)
}

// handleAwaitingApproval
// AwaitingApproval  --> Pending
// AwaitingApproval --> ApprovalReject
func (h *JobScheduler) handleAwaitingApproval(job *kusciaapisv1alpha1.KusciaJob) (needUpdateStatus bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	defer updateJobTime(now, job)
	// handle stage command, check if the stage command matches the phase of job
	if hasReconciled, err := h.handleStageCommand(now, job); err != nil || hasReconciled {
		return hasReconciled, err
	}

	// Check whether Auto Approval
	if ok, _ := common.SelfClusterIsInitiator(h.domainLister, job); !ok { //not initiator
		if !h.enableWorkloadApprove {
			ownP, _, _ := h.getAllParties(job)
			for p := range ownP {
				if job.Status.ApproveStatus == nil {
					job.Status.ApproveStatus = make(map[string]kusciaapisv1alpha1.JobApprovePhase)
				}
				job.Status.ApproveStatus[p] = kusciaapisv1alpha1.JobAccepted
				needUpdateStatus = true
			}
		}
	}

	// Only inter connection job need approval
	// the logic of handle awaiting approval is no different between  self as initiator or as partner
	// all partner have approval accept
	if ok, _ := h.allApprovalAccept(job); ok {
		// set AwaitingApproval --> Pending
		job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
		needUpdateStatus = true
	}
	// some partner have approval reject
	if ok, p, _ := h.someApprovalReject(job); ok {
		// set AwaitingApproval --> Pending
		job.Status.Phase = kusciaapisv1alpha1.KusciaJobApprovalReject
		job.Status.Reason = fmt.Sprintf("Party: %s approval reject.", p)
		needUpdateStatus = true
	}

	return needUpdateStatus, nil
}

// ApprovalRejectHandler will handle kuscia job in ApprovalReject phase.
type ApprovalRejectHandler struct {
	*JobScheduler
}

// NewApprovalRejectHandler return ApprovalRejectHandler to handle ApprovalReject kuscia job.
func NewApprovalRejectHandler(deps *Dependencies) *ApprovalRejectHandler {
	return &ApprovalRejectHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
func (h *ApprovalRejectHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.handleApprovalReject(kusciaJob)
}

// handleApprovalReject
// ApprovalReject is completed phase
func (h *ApprovalRejectHandler) handleApprovalReject(job *kusciaapisv1alpha1.KusciaJob) (needUpdateStatus bool, err error) {
	if job.Status.CompletionTime == nil {
		now := metav1.Now()
		job.Status.CompletionTime = &now
		needUpdateStatus = true
	}
	return
}
