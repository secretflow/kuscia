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
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// PendingHandler will handle kuscia job in "" or Pending phase.
type PendingHandler struct {
	*JobScheduler
}

// NewPendingHandler return PendingHandler to handle Pending kuscia job.
func NewPendingHandler(deps *Dependencies) *PendingHandler {
	return &PendingHandler{
		JobScheduler: NewJobScheduler(deps),
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
func (h *PendingHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	return h.handlePending(kusciaJob)
}

// handlePending
// pending --> running
// Pending --> Failed
func (h *PendingHandler) handlePending(job *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	defer updateJobTime(now, job)
	// handle stage command, check if the stage command matches the phase of job
	if hasReconciled, err := h.handleStageCommand(now, job); err != nil || hasReconciled {
		return hasReconciled, err
	}
	// the logic of handle pending status is no different between  self as initiator or as partner
	// all partner have been created success
	if isBFIAInterConnJob(h.namespaceLister, job) { // BFIA must checkout Start Stage
		// bfia logic
		// the initiator need to start job once
		isInitatior := utilsres.SelfClusterAsInitiator(h.namespaceLister, job.Spec.Initiator, job.Annotations)

		if isInitatior {
			if utilsres.AllJobPartiesHaveStage(job, kusciaapisv1alpha1.JobCreateStageSucceeded) {
				nlog.Infof("Job %s all parties create success, check duplicate start", job.Name)
				stageCmd, _, ok := h.getStageCmd(job)
				if !ok || kusciaapisv1alpha1.JobStage(stageCmd) != kusciaapisv1alpha1.JobStartStage {
					// set stage and trigger to reconcile
					nlog.Infof("Set stage label to trigger job %s start", job.Name)
					if err := h.setJobStage(job.Name, job.Spec.Initiator, string(kusciaapisv1alpha1.JobStartStage)); err != nil {
						nlog.Errorf("Set job stage to 'start' failed, error: %s.", err.Error())
						return true, err
					}
					return false, nil
				}
			} else if ok, p, _ := h.somePartyCreateFailed(job); ok {
				// set Pending --> Failed
				job.Status.Reason = fmt.Sprintf("Party: %s create failed.", p)
				job.Status.Phase = kusciaapisv1alpha1.KusciaJobFailed
				return true, nil
			}
		}

		// initiator need to check every party start successfully
		// partner only need to focus its own start
		if isInitatior {
			if utilsres.AllJobPartiesHaveStage(job, kusciaapisv1alpha1.JobStartStageSucceeded) {
				// set Pending --> Running
				job.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
				return true, nil
			}
			// some partner have been started failed
			if ok, p, _ := h.somePartyStartFailed(job); ok {
				// set Pending --> Failed
				job.Status.Reason = fmt.Sprintf("Party: %s start failed.", p)
				job.Status.Phase = kusciaapisv1alpha1.KusciaJobFailed
				return true, nil
			}
		} else {
			// set Pending --> Running
			job.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
			return true, nil
		}

	} else {
		// normal logic
		if ok, _ := h.allPartyCreateSuccess(job); ok {
			// set Pending --> Running
			job.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
			return true, nil
		}
		// some partner have been created failed
		if ok, p, _ := h.somePartyCreateFailed(job); ok {
			// set Pending --> Failed
			job.Status.Reason = fmt.Sprintf("Party: %s start failed.", p)
			job.Status.Phase = kusciaapisv1alpha1.KusciaJobFailed
			return true, nil
		}
	}

	return false, nil
}
