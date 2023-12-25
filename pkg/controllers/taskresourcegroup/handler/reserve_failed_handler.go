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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// ReserveFailedHandler is used to handle task resource group which phase is reserve failed.
type ReserveFailedHandler struct {
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
	podLister    listers.PodLister
	trLister     kuscialistersv1alpha1.TaskResourceLister
}

// NewReserveFailedHandler returns a ReservedFailedHandler instance.
func NewReserveFailedHandler(deps *Dependencies) *ReserveFailedHandler {
	return &ReserveFailedHandler{
		kubeClient:   deps.KubeClient,
		kusciaClient: deps.KusciaClient,
		podLister:    deps.PodLister,
		trLister:     deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *ReserveFailedHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	var trs []*kusciaapisv1alpha1.TaskResource
	now := metav1.Now().Rfc3339Copy()
	partySet := make(map[string]struct{})
	for _, party := range trg.Spec.Parties {
		if _, exist := partySet[party.DomainID]; exist {
			continue
		}

		trs, err = h.trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil {
			cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed)
			needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, corev1.ConditionFalse, fmt.Sprintf("List task resources failed, %v", err.Error()))
			return true, err
		}

		for _, tr := range trs {
			trCopy := tr.DeepCopy()
			trCopy.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserving
			trCopy.Status.LastTransitionTime = &now
			trCond := utilsres.GetTaskResourceCondition(&trCopy.Status, kusciaapisv1alpha1.TaskResourceCondReserving)
			trCond.Status = corev1.ConditionTrue
			trCond.LastTransitionTime = &now
			trCond.Reason = "Retry to reserve resource"
			if err = utilsres.PatchTaskResource(context.Background(), h.kusciaClient, utilsres.ExtractTaskResourceStatus(tr), utilsres.ExtractTaskResourceStatus(trCopy)); err != nil {
				err = fmt.Errorf("patch party task resource %v/%v failed, %v", trCopy.Namespace, trCopy.Name, err.Error())
				return false, err
			}
		}
		partySet[party.DomainID] = struct{}{}
	}

	if utilsres.IsExistingTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed, corev1.ConditionFalse) {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesListed)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, corev1.ConditionTrue, "")
	}

	if err = updatePodAnnotations(trg.Name, h.podLister, h.kubeClient); err != nil {
		cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.PodAnnotationUpdated)
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, corev1.ConditionFalse, fmt.Sprintf("Update pod annotation failed, %v", err.Error()))
		return needUpdate, err
	}

	trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserving
	trg.Status.RetryCount++
	trg.Status.LastTransitionTime = &now
	return true, nil
}
