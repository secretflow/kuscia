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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// ReservingHandler is used to handle task resource group which phase is reserving.
type ReservingHandler struct {
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
	podLister    listers.PodLister
	trLister     kuscialistersv1alpha1.TaskResourceLister
}

// NewReservingHandler returns a ReservingHandler instance.
func NewReservingHandler(deps *Dependencies) *ReservingHandler {
	return &ReservingHandler{
		kubeClient:   deps.KubeClient,
		kusciaClient: deps.KusciaClient,
		podLister:    deps.PodLister,
		trLister:     deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *ReservingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	if needUpdate, err = h.updatePodAnnotations(now, trg); err != nil {
		nlog.Error(err)
		return true, err
	}

	return h.summarizeTaskResourcesInfo(now, trg)
}

func (h *ReservingHandler) updatePodAnnotations(now metav1.Time, trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	if err = updatePodAnnotations(trg.Name, h.podLister, h.kubeClient); err != nil {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.PodAnnotationUpdated)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("Update pod annotation failed, %v", err.Error()))
		return needUpdate, err
	}

	if utilsres.IsExistingTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.PodAnnotationUpdated, v1.ConditionFalse) {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.PodAnnotationUpdated)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionTrue, "")
		return needUpdate, nil
	}
	return false, nil
}

func (h *ReservingHandler) summarizeTaskResourcesInfo(now metav1.Time, trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	var trs []*kusciaapisv1alpha1.TaskResource
	var trsCount, reservedCount, failedCount int
	partySet := make(map[string]struct{})
	for _, party := range trg.Spec.Parties {
		if _, exist := partySet[party.DomainID]; exist {
			continue
		}

		trs, err = h.trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil {
			cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed)
			needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("List task resources failed, %v", err.Error()))
			return needUpdate, err
		}

		for _, tr := range trs {
			trsCount++
			if tr.Status.Phase == kusciaapisv1alpha1.TaskResourcePhaseReserved {
				reservedCount++
			}

			if tr.Status.Phase == kusciaapisv1alpha1.TaskResourcePhaseFailed {
				failedCount++
			}
		}
		partySet[party.DomainID] = struct{}{}
	}

	if utilsres.IsExistingTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed, v1.ConditionFalse) {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionTrue, "")
	}

	if trg.Spec.MinReservedMembers > len(trg.Spec.Parties) {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		trg.Status.LastTransitionTime = &now
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesReserved)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("Task resource group min reserved member %v is greater than total parties number %v", trg.Spec.MinReservedMembers, len(trg.Spec.Parties)))
		return needUpdate, nil
	}

	if trg.Spec.MinReservedMembers > trsCount {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		trg.Status.LastTransitionTime = &now
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesReserved)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("Task resource group min reserved member %v is greater than task resources count %v", trg.Spec.MinReservedMembers, trsCount))
		return needUpdate, nil
	}

	if reservedCount >= trg.Spec.MinReservedMembers {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserved
		trg.Status.LastTransitionTime = &now
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesReserved)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionTrue, "")
		return needUpdate, nil
	}

	if trg.Spec.MinReservedMembers > len(trg.Spec.Parties)-failedCount {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesReserved)
		// patch all party status phase to failed.
		trCondReason := "Task resource group state changed to reserve-failed, so set the task resource status to failed"
		if err = patchTaskResourceStatus(trg, kusciaapisv1alpha1.TaskResourcePhaseFailed, kusciaapisv1alpha1.TaskResourceCondFailed, trCondReason, h.kusciaClient, h.trLister); err != nil {
			needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("Patch task resources status failed, %v", err.Error()))
			return needUpdate, err
		}

		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed
		if trg.Labels != nil && trg.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) {
			trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		}
		trg.Status.LastTransitionTime = &now
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse,
			fmt.Sprintf("The remaining no-failed parties count %v is less than the schedulable threshold %v", len(trg.Spec.Parties)-failedCount, trg.Spec.MinReservedMembers))
		return needUpdate, nil
	}
	return needUpdate, nil
}
