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
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/dependencies"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// FailedHandler is used to handle kuscia task which phase is failed.
type FailedHandler struct {
	*FinishedHandler
	kusciaClient kusciaclientset.Interface
	trgLister    kuscialistersv1alpha1.TaskResourceGroupLister
}

// NewFailedHandler returns a FailedHandler instance.
func NewFailedHandler(deps *dependencies.Dependencies, finishedHandler *FinishedHandler) *FailedHandler {
	return &FailedHandler{
		FinishedHandler: finishedHandler,
		kusciaClient:    deps.KusciaClient,
		trgLister:       deps.TrgLister,
	}
}

// Handle is used to perform the real logic.
func (h *FailedHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	h.setTaskResourceGroupFailed(kusciaTask)
	setPartyTaskStatusFailed(kusciaTask)
	needUpdate, err := h.FinishedHandler.Handle(kusciaTask)
	if err != nil {
		return false, err
	}
	metrics.TaskResultStats.WithLabelValues(metrics.Failed).Inc()
	return needUpdate, nil
}

func (h *FailedHandler) setTaskResourceGroupFailed(kusciaTask *kusciaapisv1alpha1.KusciaTask) {
	trg, err := h.trgLister.Get(kusciaTask.Name)
	if err != nil {
		nlog.Warnf("Get task resource group %v failed, skip setting its status to failed, %v", kusciaTask.Name, err)
		return
	}

	if trg.Status.Phase != kusciaapisv1alpha1.TaskResourceGroupPhaseFailed &&
		trg.Status.Phase != kusciaapisv1alpha1.TaskResourceGroupPhaseReserved {
		now := metav1.Now().Rfc3339Copy()
		trgCopy := trg.DeepCopy()
		trgCopy.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		trgCopy.Status.LastTransitionTime = &now
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trgCopy.Status, kusciaapisv1alpha1.DependentTaskFailed)
		utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionTrue, "")

		err = utilsres.PatchTaskResourceGroupStatus(context.Background(), h.kusciaClient, trg, trgCopy)
		if err != nil {
			nlog.Warnf("Failed to set task resource group %v to failed and retry, %v", trgCopy.Name, err)
		}
	}
}

func setPartyTaskStatusFailed(kusciaTask *kusciaapisv1alpha1.KusciaTask) {
	// if task failed, set the all party task status to failed
	if len(kusciaTask.Status.PartyTaskStatus) == 0 {
		for _, party := range kusciaTask.Spec.Parties {
			kusciaTask.Status.PartyTaskStatus = append(kusciaTask.Status.PartyTaskStatus,
				kusciaapisv1alpha1.PartyTaskStatus{
					DomainID: party.DomainID,
					Role:     party.Role,
					Phase:    kusciaapisv1alpha1.TaskFailed,
				})
		}
		return
	}

	for idx, status := range kusciaTask.Status.PartyTaskStatus {
		if status.Phase == kusciaapisv1alpha1.TaskPending || status.Phase == kusciaapisv1alpha1.TaskRunning {
			kusciaTask.Status.PartyTaskStatus[idx].Phase = kusciaapisv1alpha1.TaskFailed
		}
	}

	// if task failed, set the all pods status to failed
	for _, status := range kusciaTask.Status.PodStatuses {
		if status.PodPhase == v1.PodPending || status.PodPhase == v1.PodRunning {
			status.PodPhase = v1.PodFailed
		}
	}
}
