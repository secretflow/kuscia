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
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
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
func (h *ReservingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	var err error
	if err = updatePodAnnotations(trg.Name, h.podLister, h.kubeClient); err != nil {
		return false, fmt.Errorf("update pod annotation failed, %v", err.Error())
	}

	var (
		now                        = metav1.Now().Rfc3339Copy()
		reservedCount, failedCount int
		trs                        []*kusciaapisv1alpha1.TaskResource
	)

	// check whether the count of parties failed or successfully reserved.
	trsCount := 0
	partySet := make(map[string]struct{})
	for _, party := range trg.Spec.Parties {
		if _, exist := partySet[party.DomainID]; exist {
			continue
		}

		trs, err = h.trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil {
			nlog.Warnf("List party %v task resource of task resource group %v failed, %v", party.DomainID, trg.Name, err)
			continue
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

	if trg.Spec.MinReservedMembers > len(trg.Spec.Parties) {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		trg.Status.LastTransitionTime = now
		reservingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondReserving)
		reservingCond.Status = v1.ConditionTrue
		reservingCond.Reason = "task resource group min reserved member is greater than total parties number"
		reservingCond.LastTransitionTime = now
		return true, nil
	}

	if trg.Spec.MinReservedMembers > trsCount {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		trg.Status.LastTransitionTime = now
		reservingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondReserving)
		reservingCond.Status = v1.ConditionTrue
		reservingCond.Reason = "task resource group min reserved member is greater than task resource count"
		reservingCond.LastTransitionTime = now
		return true, nil
	}

	if reservedCount >= trg.Spec.MinReservedMembers {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserved
		trg.Status.LastTransitionTime = now
		reservingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondReserving)
		reservingCond.Status = v1.ConditionTrue
		reservingCond.Reason = "Min member pods reserve resource"
		reservingCond.LastTransitionTime = now
		return true, nil
	}

	if trg.Spec.MinReservedMembers > len(trg.Spec.Parties)-failedCount {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed
		trg.Status.LastTransitionTime = now
		reservingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondReserving)
		reservingCond.Status = v1.ConditionTrue
		reservingCond.Reason = "The count of failed reserved parties exceeds the schedulable threshold"
		reservingCond.LastTransitionTime = now

		// patch all party status phase to failed.
		condReason := "Task resource group state is reserve failed and change the task resource state to failed"
		if err = patchTaskResourceStatus(trg, kusciaapisv1alpha1.TaskResourcePhaseFailed, kusciaapisv1alpha1.TaskResourceCondFailed, condReason, h.kusciaClient, h.trLister); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
