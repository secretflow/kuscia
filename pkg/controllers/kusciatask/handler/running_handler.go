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
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	taskDefaultLifecycle = 600 * time.Second
)

const (
	defaultPullImageRetryCount = 3

	errImagePull        = "ErrImagePull"
	errImagePullBackOff = "ErrImagePullBackOff"
)

var podFailedContainerReason = map[string]bool{
	"CrashLoopBackOff":           true,
	"ImageInspectError":          true,
	"ErrImageNeverPull":          true,
	"RegistryUnavailable":        true,
	"InvalidImageName":           true,
	"CreateContainerConfigError": true,
	"CreateContainerError":       true,
	"PreStartHookError":          true,
	"PostStartHookError":         true,
}

type partyStatus string

const (
	partyPending   partyStatus = "Pending"
	partyRunning   partyStatus = "Running"
	partySucceeded partyStatus = "Succeeded"
	partyFailed    partyStatus = "Failed"
)

// RunningHandler is used to handle kuscia task which phase is running.
type RunningHandler struct {
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
	trgLister    kuscialistersv1alpha1.TaskResourceGroupLister
	podsLister   corelisters.PodLister
	nsLister     corelisters.NamespaceLister
}

// NewRunningHandler returns a RunningHandler instance.
func NewRunningHandler(deps *Dependencies) *RunningHandler {
	return &RunningHandler{
		kubeClient:   deps.KubeClient,
		kusciaClient: deps.KusciaClient,
		trgLister:    deps.TrgLister,
		podsLister:   deps.PodsLister,
		nsLister:     deps.NamespacesLister,
	}
}

// Handle is used to perform the real logic.
func (h *RunningHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	now := metav1.Now().Rfc3339Copy()
	trg, err := h.trgLister.Get(kusciaTask.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			trg, err = h.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(context.Background(), kusciaTask.Name, metav1.GetOptions{})
		}

		if err != nil {
			nlog.Errorf("Can't find task resource group %v of kuscia task %v, %v", kusciaTask.Name, kusciaTask.Name, err)
			kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskFailed
			kusciaTask.Status.Message = fmt.Sprintf("Get task resource group failed, %v", err)
			return true, nil
		}
	}

	taskStatus := kusciaTask.Status.DeepCopy()
	setTaskStatusPhase(taskStatus, trg)

	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserved {
		h.reconcileTaskStatus(taskStatus, trg)
		h.refreshPodStatuses(taskStatus.PodStatuses)
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	} else {
		h.refreshPodStatuses(taskStatus.PodStatuses)
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	}

	return false, nil
}

func (h *RunningHandler) reconcileTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) {
	var (
		succeededPartyCount = 0
		failedPartyCount    = 0
		runningPartyCount   = 0
		pendingPartyCount   = 0
	)

	partyTaskStatuses := h.buildPartyTaskStatus(taskStatus, trg)
	taskStatus.PartyTaskStatus = partyTaskStatuses
	for _, s := range partyTaskStatuses {
		switch s.Phase {
		case kusciaapisv1alpha1.TaskSucceeded:
			succeededPartyCount++
		case kusciaapisv1alpha1.TaskFailed:
			failedPartyCount++
		case kusciaapisv1alpha1.TaskRunning:
			runningPartyCount++
		default:
			pendingPartyCount++
		}
	}

	validPartyCount := len(trg.Spec.Parties)
	minReservedMembers := trg.Spec.MinReservedMembers
	if !utilsres.SelfClusterAsInitiator(h.nsLister, trg.Spec.Initiator, nil) {
		minReservedMembers = minReservedMembers - h.getOuterInterConnPartyCount(trg)
		validPartyCount = validPartyCount - h.getOuterInterConnPartyCount(trg)
	}

	if minReservedMembers > validPartyCount-failedPartyCount {
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Message = fmt.Sprintf("The remaining non-failed parties counts %v is less than the task success threshold %v", len(trg.Spec.Parties)-failedPartyCount, trg.Spec.MinReservedMembers)
		return
	}

	if succeededPartyCount >= minReservedMembers {
		taskStatus.Phase = kusciaapisv1alpha1.TaskSucceeded
		return
	}

	if runningPartyCount > 0 {
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		return
	}

	// when runningPartyCount is equal to 0 and pendingPartyCount is greater than 0
	// if task exceed the max lifecycle, set the task to failed
	if pendingPartyCount > 0 {
		taskLifecycle := taskDefaultLifecycle
		if trg.Spec.LifecycleSeconds != 0 {
			taskLifecycle = time.Duration(trg.Spec.LifecycleSeconds) * time.Second
		}

		expiredTime := taskStatus.StartTime.Add(taskLifecycle)
		if metav1.Now().After(expiredTime) {
			taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
			taskStatus.Message = fmt.Sprintf("Pending parties are not scheduled during lifecycle %v", taskLifecycle)
			return
		}
	}
}

func (h *RunningHandler) buildPartyTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) []kusciaapisv1alpha1.PartyTaskStatus {
	partyTaskStatuses := make([]kusciaapisv1alpha1.PartyTaskStatus, 0)
	for _, party := range trg.Spec.Parties {
		if utilsres.IsOuterBFIAInterConnDomain(h.nsLister, party.DomainID) {
			outerPartyTaskStatus := kusciaapisv1alpha1.PartyTaskStatus{
				DomainID: party.DomainID,
				Role:     party.Role,
			}

			for _, s := range taskStatus.PartyTaskStatus {
				if s.DomainID == party.DomainID && s.Role == party.Role {
					outerPartyTaskStatus.Phase = s.Phase
					outerPartyTaskStatus.Message = s.Message
					break
				}
			}
			partyTaskStatuses = append(partyTaskStatuses, outerPartyTaskStatus)
			continue
		}

		partyTaskStatus := kusciaapisv1alpha1.PartyTaskStatus{
			DomainID: party.DomainID,
			Role:     party.Role,
		}

		switch h.getPartyTaskStatus(taskStatus, party) {
		case partyFailed:
			partyTaskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		case partySucceeded:
			partyTaskStatus.Phase = kusciaapisv1alpha1.TaskSucceeded
		case partyRunning:
			partyTaskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		default:
			partyTaskStatus.Phase = kusciaapisv1alpha1.TaskPending
		}

		partyTaskStatuses = append(partyTaskStatuses, partyTaskStatus)
	}
	return partyTaskStatuses
}

func (h *RunningHandler) getOuterInterConnPartyCount(trg *kusciaapisv1alpha1.TaskResourceGroup) int {
	outerPartyCount := 0
	for _, party := range trg.Spec.Parties {
		if utilsres.IsOuterBFIAInterConnDomain(h.nsLister, party.DomainID) {
			outerPartyCount++
		}
	}
	return outerPartyCount
}

func (h *RunningHandler) getPartyTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, party kusciaapisv1alpha1.TaskResourceGroupParty) partyStatus {
	for _, pts := range taskStatus.PartyTaskStatus {
		if pts.DomainID == party.DomainID && pts.Role == party.Role {
			if pts.Phase == kusciaapisv1alpha1.TaskSucceeded {
				return partySucceeded
			}

			if pts.Phase == kusciaapisv1alpha1.TaskFailed {
				return partyFailed
			}
		}
	}

	runningPodCount := 0
	failedPodCount := 0
	succeededPodCount := 0
	for _, pp := range party.Pods {
		pod, err := h.podsLister.Pods(party.DomainID).Get(pp.Name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				pod, err = h.kubeClient.CoreV1().Pods(party.DomainID).Get(context.Background(), pp.Name, metav1.GetOptions{})
			}

			if err != nil {
				nlog.Errorf("Can't get party %v pod %v, %v", party.DomainID, pp.Name, err)
				failedPodCount++
				continue
			}
		}

		if pod == nil {
			failedPodCount++
			continue
		}

		switch pod.Status.Phase {
		case v1.PodPending:
			if failed := isPodFailed(&pod.Status); failed {
				failedPodCount++
			}
		case v1.PodRunning:
			if failed := isPodFailed(&pod.Status); failed {
				failedPodCount++
			} else {
				runningPodCount++
			}
		case v1.PodSucceeded:
			succeededPodCount++
		case v1.PodFailed:
			failedPodCount++
		default:
			nlog.Warnf("Unknown pod %v phase %v", fmt.Sprintf(party.DomainID, pp.Name), pod.Status.Phase)
			failedPodCount++
		}
	}

	podsCount := len(party.Pods)
	minReservedPods := party.MinReservedPods
	if minReservedPods == 0 {
		minReservedPods = podsCount
	}

	if minReservedPods > podsCount-failedPodCount {
		return partyFailed
	}

	if succeededPodCount >= minReservedPods {
		return partySucceeded
	}

	if runningPodCount > 0 {
		return partyRunning
	}

	return partyPending
}

func (h *RunningHandler) refreshPodStatuses(podStatuses map[string]*kusciaapisv1alpha1.PodStatus) {
	for _, st := range podStatuses {
		if st.PodPhase == v1.PodSucceeded || st.PodPhase == v1.PodFailed {
			continue
		}

		ns := st.Namespace
		name := st.PodName
		pod, err := h.podsLister.Pods(ns).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				pod, err = h.kubeClient.CoreV1().Pods(ns).Get(context.Background(), name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					st.PodPhase = v1.PodFailed
					st.Reason = "PodNotExist"
					st.Message = "Does not find the pod"
					continue
				}
			}

			if err != nil {
				st.PodPhase = v1.PodFailed
				st.Reason = "GetPodFailed"
				st.Message = err.Error()
			}
			continue
		}

		st.PodPhase = pod.Status.Phase
		st.NodeName = pod.Spec.NodeName
		st.Reason = pod.Status.Reason
		st.Message = pod.Status.Message

		// check pod container terminated state
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				if st.Reason == "" && cs.State.Terminated.Reason != "" {
					st.Reason = cs.State.Terminated.Reason
				}
				if cs.State.Terminated.Message != "" {
					st.TerminationLog = fmt.Sprintf("container[%v] terminated state reason %q, message: %q", cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.Message)
				}
			} else if cs.LastTerminationState.Terminated != nil {
				if st.Reason == "" && cs.LastTerminationState.Terminated.Reason != "" {
					st.Reason = cs.LastTerminationState.Terminated.Reason
				}
				if cs.LastTerminationState.Terminated.Message != "" {
					st.TerminationLog = fmt.Sprintf("container[%v] last terminated state reason %q, message: %q", cs.Name, cs.LastTerminationState.Terminated.Reason, cs.LastTerminationState.Terminated.Message)
				}
			}

			// set terminated log from one of containers
			if st.TerminationLog != "" {
				break
			}
		}

		if st.Reason != "" && st.Message != "" {
			return
		}

		// Check if the pod has been scheduled
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse {
				if st.Reason == "" {
					st.Reason = cond.Reason
				}

				if st.Message == "" {
					st.Message = cond.Message
				}
				return
			}
		}

		// check pod container waiting state
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				if st.Reason == "" && cs.State.Waiting.Reason != "" {
					st.Reason = cs.State.Waiting.Reason
				}

				if cs.State.Waiting.Message != "" {
					st.Message = fmt.Sprintf("container[%v] waiting state reason: %q, message: %q", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
				}
			}

			if st.Message != "" {
				return
			}
		}
	}
}

func getTaskLifecycle(kusciaTask *kusciaapisv1alpha1.KusciaTask) time.Duration {
	taskLifecycle := taskDefaultLifecycle
	if kusciaTask.Spec.ScheduleConfig.LifecycleSeconds > 0 {
		taskLifecycle = time.Duration(kusciaTask.Spec.ScheduleConfig.LifecycleSeconds) * time.Second
	}
	return taskLifecycle
}

func isPodFailed(ps *v1.PodStatus) bool {
	for _, cond := range ps.Conditions {
		if cond.Type == v1.ContainersReady && cond.Status == v1.ConditionFalse {
			for _, cs := range ps.ContainerStatuses {
				if cs.State.Waiting != nil {
					switch cs.State.Waiting.Reason {
					case errImagePull, errImagePullBackOff:
						// currently, when image pull failed, the RestartCount will not be changed
						// in the future, relevant codes will be modified to change RestartCount value
						if cs.RestartCount > defaultPullImageRetryCount {
							return true
						}
					default:
						return podFailedContainerReason[cs.State.Waiting.Reason]
					}
				}
			}
		}
	}
	return false
}

func fillTaskCondition(status *kusciaapisv1alpha1.KusciaTaskStatus) {
	var cond *kusciaapisv1alpha1.KusciaTaskCondition
	switch status.Phase {
	case kusciaapisv1alpha1.TaskRunning:
		cond, _ = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondRunning, true)
		cond.Status = v1.ConditionTrue
	case kusciaapisv1alpha1.TaskFailed:
		cond, _ = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondSuccess, true)
		cond.Status = v1.ConditionFalse
	case kusciaapisv1alpha1.TaskSucceeded:
		cond, _ = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondSuccess, true)
		cond.Status = v1.ConditionTrue
	}

	if status.LastReconcileTime != nil {
		cond.LastTransitionTime = status.LastReconcileTime
	} else {
		now := metav1.Now().Rfc3339Copy()
		cond.LastTransitionTime = &now
	}
}

func setTaskStatusPhase(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) {
	switch trg.Status.Phase {
	case "",
		kusciaapisv1alpha1.TaskResourceGroupPhasePending,
		kusciaapisv1alpha1.TaskResourceGroupPhaseCreating,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserving,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed,
		kusciaapisv1alpha1.TaskResourceGroupPhaseReserved:
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		taskStatus.Message = buildTaskStatusMessage(trg)
	case kusciaapisv1alpha1.TaskResourceGroupPhaseFailed:
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Reason = "TaskResourceGroupPhaseFailed"
		taskStatus.Message = buildTaskStatusMessage(trg)
	default:
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Reason = "TaskResourceGroupPhaseUnknown"
		taskStatus.Message = "Task resource group status phase is unknown"
	}
}

func buildTaskStatusMessage(trg *kusciaapisv1alpha1.TaskResourceGroup) string {
	for _, cond := range trg.Status.Conditions {
		if cond.Status != v1.ConditionTrue && cond.Reason != "" {
			return cond.Reason
		}
	}
	return ""
}
