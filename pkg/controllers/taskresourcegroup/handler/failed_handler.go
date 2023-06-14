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

// FailedHandler is used to handle task resource group which phase is failed.
type FailedHandler struct {
	kusciaClient kusciaclientset.Interface
	trLister     kuscialistersv1alpha1.TaskResourceLister
}

// NewFailedHandler returns a FailedHandler instance.
func NewFailedHandler(deps *Dependencies) *FailedHandler {
	return &FailedHandler{
		kusciaClient: deps.KusciaClient,
		trLister:     deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *FailedHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	failedCond, found := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondFailed)
	if found {
		nlog.Infof("Task resource group status has failed condition, skip to handle it")
		return false, nil
	}

	condReason := "Task resource group state change to failed"
	if err := patchTaskResourceStatus(trg, kusciaapisv1alpha1.TaskResourcePhaseFailed, kusciaapisv1alpha1.TaskResourceCondFailed, condReason, h.kusciaClient, h.trLister); err != nil {
		return false, err
	}

	now := metav1.Now().Rfc3339Copy()
	trg.Status.CompletionTime = &now
	failedCond.Status = v1.ConditionTrue
	failedCond.Reason = "Task resource group status phase is failed"
	failedCond.LastTransitionTime = now
	return true, nil
}
