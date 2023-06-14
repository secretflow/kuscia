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

//nolint:dulp
package handler

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// ReservedHandler is used to handle task resource group which phase is reserved.
type ReservedHandler struct {
	kusciaClient kusciaclientset.Interface
	trLister     kuscialistersv1alpha1.TaskResourceLister
}

// NewReservedHandler returns a ReservedHandler instance.
func NewReservedHandler(deps *Dependencies) *ReservedHandler {
	return &ReservedHandler{
		kusciaClient: deps.KusciaClient,
		trLister:     deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *ReservedHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	reservedCond, found := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondReserved)
	if found {
		nlog.Infof("Task resource group status has reserved condition, skip to handle it")
		return false, nil
	}

	condReason := "Pods related to task resource can be scheduled"
	if err := patchTaskResourceStatus(trg, kusciaapisv1alpha1.TaskResourcePhaseSchedulable, kusciaapisv1alpha1.TaskResourceCondSchedulable, condReason, h.kusciaClient, h.trLister); err != nil {
		return false, err
	}

	now := metav1.Now().Rfc3339Copy()
	trg.Status.CompletionTime = &now
	reservedCond.Status = v1.ConditionTrue
	reservedCond.Reason = "Finish patch task resource status phase to schedulable"
	reservedCond.LastTransitionTime = now
	return true, nil
}
