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
	"reflect"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

// RunningHandler is used to handle kuscia task which phase is running.
type RunningHandler struct {
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
	trgLister    kuscialistersv1alpha1.TaskResourceGroupLister
	trLister     kuscialistersv1alpha1.TaskResourceLister
	podsLister   corelisters.PodLister
}

// NewRunningHandler returns a RunningHandler instance.
func NewRunningHandler(deps *Dependencies) *RunningHandler {
	return &RunningHandler{
		kubeClient:   deps.KubeClient,
		kusciaClient: deps.KusciaClient,
		trgLister:    deps.TrgLister,
		trLister:     deps.TrLister,
		podsLister:   deps.PodsLister,
	}
}

// Handle is used to perform the real logic.
func (h *RunningHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	taskStatus := kusciaTask.Status.DeepCopy()
	trg, err := h.trgLister.Get(kusciaTask.Name)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			trg, err = h.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(context.Background(), kusciaTask.Name, metav1.GetOptions{})
			if err != nil && k8serrors.IsNotFound(err) {
				taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
				taskStatus.Reason = "TaskResourceGroupNotFound"
				taskStatus.Message = err.Error()
				return true, err
			}
		}

		if err != nil {
			taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
			taskStatus.Reason = "GetTaskResourceGroupFailed"
			taskStatus.Message = err.Error()
			return true, err
		}
	}

	setTaskStatusPhaseByTrg(taskStatus, trg)

	now := metav1.Now().Rfc3339Copy()
	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserved {
		h.checkAndUpdateTaskStatus(taskStatus, trg)
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	} else {
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	}

	return false, nil
}

func (h *RunningHandler) checkAndUpdateTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) {
	h.refreshPodStatuses(taskStatus.PodStatuses)

	trgMinReservedMembers := trg.Spec.MinReservedMembers
	if trgMinReservedMembers == 0 {
		trgMinReservedMembers = len(trg.Spec.Parties)
	}

	var (
		partySet            = make(map[string]struct{})
		succeededPartyCount = 0
		failedPartyCount    = 0
		findTaskPods        bool
	)

	for _, party := range trg.Spec.Parties {
		if _, exist := partySet[party.DomainID]; exist {
			continue
		}

		trs, err := h.trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil {
			taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
			taskStatus.Reason = "ListTaskResourceFailed"
			taskStatus.Message = err.Error()
			return
		}
		// there is a scenario: party domainID may be same
		// one party should correspond to one task resource
		for _, tr := range trs {
			minReservedPods := tr.Spec.MinReservedPods
			if minReservedPods == 0 {
				minReservedPods = len(tr.Spec.Pods)
			}

			podCount := 0
			podPhase := make(map[v1.PodPhase]int)
			for _, trPod := range tr.Spec.Pods {
				pod, err := h.podsLister.Pods(tr.Namespace).Get(trPod.Name)
				if err != nil {
					continue
				}
				podPhase[pod.Status.Phase]++
				podCount++
				findTaskPods = true
			}

			if podCount < minReservedPods {
				failedPartyCount++
				continue
			}

			if podPhase[v1.PodFailed] > 0 {
				failedPartyCount++
				continue
			}

			if podPhase[v1.PodSucceeded] >= minReservedPods {
				succeededPartyCount++
			}
		}

		partySet[party.DomainID] = struct{}{}
	}

	if len(trg.Spec.Parties)-failedPartyCount < trgMinReservedMembers {
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Reason = "TaskFailed"
		taskStatus.Message = "The succeeded party number is less than minReservedMembers"
		return
	}

	if succeededPartyCount >= trgMinReservedMembers {
		taskStatus.Phase = kusciaapisv1alpha1.TaskSucceeded
		return
	}

	if findTaskPods {
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskRunning"
		taskStatus.Message = "Find pods for task"
		return
	}

	taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
	taskStatus.Reason = "TaskFailed"
	taskStatus.Message = "Can't find pods for task"
}

func (h *RunningHandler) refreshPodStatuses(podStatuses map[string]*kusciaapisv1alpha1.PodStatus) {
	for _, st := range podStatuses {
		ns := st.Namespace
		name := st.PodName
		pod, err := h.podsLister.Pods(ns).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				pod, err = h.kubeClient.CoreV1().Pods(ns).Get(context.Background(), name, metav1.GetOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						st.PodPhase = v1.PodFailed
						st.Reason = "PodNotExist"
						st.Message = "Does not find the pod"
					}
				}
			} else {
				st.PodPhase = v1.PodFailed
				st.Reason = "GetPodFailed"
				st.Message = err.Error()
			}
			continue
		}

		st.PodPhase = pod.Status.Phase
		st.Reason = pod.Status.Reason
		st.Message = pod.Status.Message
		st.NodeName = pod.Spec.NodeName

		// generate TerminationLog
		if len(pod.Status.ContainerStatuses) >= 1 {
			c := pod.Status.ContainerStatuses[0]
			if c.State.Terminated != nil {
				st.TerminationLog = c.State.Terminated.Message
			} else if c.LastTerminationState.Terminated != nil {
				st.TerminationLog = c.LastTerminationState.Terminated.Message
			} else {
				st.TerminationLog = ""
			}
		}
	}
}

func fillTaskCondition(status *kusciaapisv1alpha1.KusciaTaskStatus) {
	var cond *kusciaapisv1alpha1.KusciaTaskCondition
	switch status.Phase {
	case kusciaapisv1alpha1.TaskRunning:
		cond = getKusciaTaskCondition(status, kusciaapisv1alpha1.TaskIsRunning)
		cond.Status = v1.ConditionTrue
	case kusciaapisv1alpha1.TaskFailed:
		cond = getKusciaTaskCondition(status, kusciaapisv1alpha1.TaskRunFailed)
		cond.Status = v1.ConditionTrue
	case kusciaapisv1alpha1.TaskSucceeded:
		cond = getKusciaTaskCondition(status, kusciaapisv1alpha1.TaskRunSuccess)
		cond.Status = v1.ConditionTrue
	}

	cond.Reason = status.Reason
	cond.Message = status.Message
	if status.LastReconcileTime != nil {
		cond.LastTransitionTime = *status.LastReconcileTime
	} else {
		now := metav1.Now().Rfc3339Copy()
		cond.LastTransitionTime = now
	}
}

func setTaskStatusPhaseByTrg(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) {
	switch trg.Status.Phase {
	case "":
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhasePending"
		taskStatus.Message = "Task resource group status phase is pending"
	case kusciaapisv1alpha1.TaskResourceGroupPhasePending:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhasePending"
		taskStatus.Message = "Task resource group status phase is pending"
	case kusciaapisv1alpha1.TaskResourceGroupPhaseCreating:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhaseCreating"
		taskStatus.Message = "Task resource group status phase is creating"
	case kusciaapisv1alpha1.TaskResourceGroupPhaseReserving:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhaseReserving"
		taskStatus.Message = "Task resource group status phase is reserving"
	case kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhaseReserveFailed"
		taskStatus.Message = "Task resource group status phase is reserve failed，waiting to retry"
	case kusciaapisv1alpha1.TaskResourceGroupPhaseReserved:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Reason = "TaskResourceGroupPhaseReserved"
		taskStatus.Message = "Task resource group status phase is reserved"
	case kusciaapisv1alpha1.TaskResourceGroupPhaseFailed:
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Reason = "TaskResourceGroupPhaseFailed"
		taskStatus.Message = "Task resource group status phase is failed"
	default:
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Reason = "TaskResourceGroupPhaseUnknown"
		taskStatus.Message = "Task resource group status phase is unknown"
	}
}
