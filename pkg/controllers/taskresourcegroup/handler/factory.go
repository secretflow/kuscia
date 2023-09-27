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
	"time"

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

// TaskResourceGroupPhaseHandler is an interface to handle task resource group.
type TaskResourceGroupPhaseHandler interface {
	Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error)
}

// Dependencies defines some parameter dependencies of functions.
type Dependencies struct {
	KubeClient      kubernetes.Interface
	KusciaClient    kusciaclientset.Interface
	NamespaceLister listers.NamespaceLister
	PodLister       listers.PodLister
	TrLister        kuscialistersv1alpha1.TaskResourceLister
}

// TaskResourceGroupPhaseHandlerFactory is a factory to get phase handler by task resource group phase.
type TaskResourceGroupPhaseHandlerFactory struct {
	TaskResourceGroupPhaseHandlerMap map[kusciaapisv1alpha1.TaskResourceGroupPhase]TaskResourceGroupPhaseHandler
}

// NewTaskResourceGroupPhaseHandlerFactory returns a TaskResourceGroupPhaseHandlerFactory instance.
func NewTaskResourceGroupPhaseHandlerFactory(deps *Dependencies) *TaskResourceGroupPhaseHandlerFactory {
	pendingHandler := NewPendingHandler(deps)
	creatingHandler := NewCreatingHandler(deps)
	reservingHandler := NewReservingHandler(deps)
	reserveFailedHandler := NewReserveFailedHandler(deps)
	reservedHandler := NewReservedHandler(deps)
	failedHandler := NewFailedHandler(deps)
	taskResourceGroupPhaseHandlerMap := map[kusciaapisv1alpha1.TaskResourceGroupPhase]TaskResourceGroupPhaseHandler{
		kusciaapisv1alpha1.TaskResourceGroupPhasePending:       pendingHandler,
		kusciaapisv1alpha1.TaskResourceGroupPhaseCreating:      creatingHandler,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserving:     reservingHandler,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed: reserveFailedHandler,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserved:      reservedHandler,
		kusciaapisv1alpha1.TaskResourceGroupPhaseFailed:        failedHandler,
	}
	return &TaskResourceGroupPhaseHandlerFactory{TaskResourceGroupPhaseHandlerMap: taskResourceGroupPhaseHandlerMap}
}

// GetTaskResourceGroupPhaseHandler is used to get TaskResourceGroupPhaseHandler by TaskResourceGroupPhase.
func (f *TaskResourceGroupPhaseHandlerFactory) GetTaskResourceGroupPhaseHandler(phase kusciaapisv1alpha1.TaskResourceGroupPhase) TaskResourceGroupPhaseHandler {
	return f.TaskResourceGroupPhaseHandlerMap[phase]
}

// updatePodAnnotations is used to update pod annotations.
func updatePodAnnotations(trgName string, podLister listers.PodLister, kubeClient kubernetes.Interface) error {
	var pods []*corev1.Pod
	podLabel := labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trgName})
	pods, err := podLister.List(podLabel)
	if err != nil {
		return err
	}

	for i, pod := range pods {
		podCopy := pod.DeepCopy()
		if podCopy.Annotations == nil {
			podCopy.Annotations = make(map[string]string)
		}
		podCopy.Annotations[common.TaskResourceReservingTimestampAnnotationKey] = metav1.Now().Format(time.RFC3339)
		oldExtractedPod := utilsres.ExtractPodAnnotations(pods[i])
		newExtractedPod := utilsres.ExtractPodAnnotations(podCopy)
		if err = utilsres.PatchPod(context.Background(), kubeClient, oldExtractedPod, newExtractedPod); err != nil {
			return err
		}
	}
	return nil
}

// patchTaskResourceStatus is used to patch task resource status.
func patchTaskResourceStatus(trg *kusciaapisv1alpha1.TaskResourceGroup,
	trPhase kusciaapisv1alpha1.TaskResourcePhase,
	trCondType kusciaapisv1alpha1.TaskResourceConditionType,
	condReason string,
	kusciaClient kusciaclientset.Interface,
	trLister kuscialistersv1alpha1.TaskResourceLister) error {
	now := metav1.Now()
	partySet := make(map[string]struct{})
	for _, party := range trg.Spec.Parties {
		if _, exist := partySet[party.DomainID]; exist {
			continue
		}

		trs, err := trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil {
			return fmt.Errorf("list party %v task resource of task resource group %v failed, %v", party.DomainID, trg.Name, err)
		}

		for _, tr := range trs {
			trCopy := tr.DeepCopy()
			trCopy.Status.Phase = trPhase
			trCopy.Status.LastTransitionTime = &now
			trCond := utilsres.GetTaskResourceCondition(&trCopy.Status, trCondType)
			trCond.LastTransitionTime = &now
			trCond.Status = corev1.ConditionTrue
			trCond.Reason = condReason

			if trPhase == kusciaapisv1alpha1.TaskResourcePhaseSchedulable || trPhase == kusciaapisv1alpha1.TaskResourcePhaseFailed {
				trCopy.Status.CompletionTime = &now
			}

			if err = utilsres.PatchTaskResource(context.Background(), kusciaClient, utilsres.ExtractTaskResourceStatus(tr), utilsres.ExtractTaskResourceStatus(trCopy)); err != nil {
				return fmt.Errorf("patch task resource %v/%v status failed, %v", party.DomainID, trCopy.Name, err.Error())
			}
		}
		partySet[party.DomainID] = struct{}{}
	}
	return nil
}
