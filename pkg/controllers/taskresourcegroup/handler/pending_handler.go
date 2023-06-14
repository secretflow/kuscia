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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// PendingHandler is used to handle task resource group which phase is pending.
type PendingHandler struct{}

// NewPendingHandler returns a PendingHandler instance.
func NewPendingHandler() *PendingHandler {
	return &PendingHandler{}
}

// Handle is used to perform the real logic.
func (h *PendingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	now := metav1.Now().Rfc3339Copy()

	trg.Status.StartTime = now
	trg.Status.LastTransitionTime = now
	pendingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondPending)
	pendingCond.Status = v1.ConditionTrue
	pendingCond.LastTransitionTime = now
	if err := validate(trg); err != nil {
		nlog.Error(err)
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		pendingCond.Reason = err.Error()
		return true, nil
	}

	trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseCreating
	pendingCond.Reason = "Task resource group is created"
	return true, nil
}

// validate is used to validate task resource group.
func validate(trg *kusciaapisv1alpha1.TaskResourceGroup) error {
	if trg.Spec.MinReservedMembers > len(trg.Spec.Parties) {
		return fmt.Errorf("task resource group %v minReservedMembers can't be greater than parties", trg.Name)
	}

	if trg.Spec.Initiator == "" {
		return fmt.Errorf("task resource group initiator %q should be one of parties", trg.Spec.Initiator)
	}

	for _, party := range trg.Spec.Parties {
		if party.DomainID == trg.Spec.Initiator {
			return nil
		}
	}

	return fmt.Errorf("task resource group initiator %q should be one of parties", trg.Spec.Initiator)

}
