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

// nolint:dulp
package handler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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
	kubeClient     kubernetes.Interface
	kusciaClient   kusciaclientset.Interface
	trgLister      kuscialistersv1alpha1.TaskResourceGroupLister
	podsLister     corelisters.PodLister
	servicesLister corelisters.ServiceLister
	nsLister       corelisters.NamespaceLister
}

// NewRunningHandler returns a RunningHandler instance.
func NewRunningHandler(deps *Dependencies) *RunningHandler {
	return &RunningHandler{
		kubeClient:     deps.KubeClient,
		kusciaClient:   deps.KusciaClient,
		trgLister:      deps.TrgLister,
		podsLister:     deps.PodsLister,
		servicesLister: deps.ServicesLister,
		nsLister:       deps.NamespacesLister,
	}
}

// Handle is used to perform the real logic.
func (h *RunningHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	now := metav1.Now().Rfc3339Copy()

	trg, err := getTaskResourceGroup(context.Background(), kusciaTask.Name, h.trgLister, h.kusciaClient)
	if err != nil {
		return false, fmt.Errorf("get task resource group %v failed, %v", kusciaTask.Name, err)
	}

	taskStatus := kusciaTask.Status.DeepCopy()
	setTaskStatusPhase(taskStatus, trg)

	refreshTaskStatus := false
	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserved ||
		(trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserving &&
			!utilsres.SelfClusterAsInitiator(h.nsLister, trg.Spec.Initiator, trg.Annotations)) {
		refreshTaskStatus = true
	}

	if refreshTaskStatus {
		h.reconcileTaskStatus(taskStatus, trg)
		refreshKtResourcesStatus(h.kubeClient, h.podsLister, h.servicesLister, taskStatus)
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	} else {
		refreshKtResourcesStatus(h.kubeClient, h.podsLister, h.servicesLister, taskStatus)
		if !reflect.DeepEqual(taskStatus, kusciaTask.Status) {
			taskStatus.LastReconcileTime = &now
			fillTaskCondition(taskStatus)
			kusciaTask.Status = *taskStatus
			return true, nil
		}
	}

	return false, nil
}

// reconcileTaskStatus update task status by gathering statistic of partyTaskStatuses
func (h *RunningHandler) reconcileTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) {
	var pendingParty, runningParty, successfulParty, failedParty []string
	partyTaskStatuses := h.buildPartyTaskStatus(taskStatus, trg)
	taskStatus.PartyTaskStatus = partyTaskStatuses
	for _, s := range partyTaskStatuses {
		switch s.Phase {
		case kusciaapisv1alpha1.TaskSucceeded:
			successfulParty = append(successfulParty, buildPartyKey(s.DomainID, s.Role))
		case kusciaapisv1alpha1.TaskFailed:
			failedParty = append(failedParty, buildPartyKey(s.DomainID, s.Role))
		case kusciaapisv1alpha1.TaskRunning:
			runningParty = append(runningParty, buildPartyKey(s.DomainID, s.Role))
		default:
			pendingParty = append(pendingParty, buildPartyKey(s.DomainID, s.Role))
		}
	}

	successfulPartyCount := len(successfulParty)
	failedPartyCount := len(failedParty)
	runningPartyCount := len(runningParty)

	validPartyCount := len(trg.Spec.Parties) + len(trg.Spec.OutOfControlledParties)
	minReservedMembers := trg.Spec.MinReservedMembers

	if minReservedMembers > validPartyCount-failedPartyCount {
		taskStatus.Phase = kusciaapisv1alpha1.TaskFailed
		taskStatus.Message = fmt.Sprintf("The remaining no-failed party task counts %v are less than the threshold %v that meets the conditions for task success. pending party[%v], running party[%v], successful party[%v], failed party[%v]",
			validPartyCount-failedPartyCount, trg.Spec.MinReservedMembers, strings.Join(pendingParty, ","), strings.Join(runningParty, ","), strings.Join(successfulParty, ","), strings.Join(failedParty, ","))
		return
	}

	if successfulPartyCount >= minReservedMembers {
		taskStatus.Phase = kusciaapisv1alpha1.TaskSucceeded
		return
	}

	if runningPartyCount > 0 {
		taskStatus.Phase = kusciaapisv1alpha1.TaskRunning
		return
	}
}

func buildPartyKey(domainID, role string) string {
	return strings.TrimSuffix(fmt.Sprintf("%v-%v", domainID, role), "-")
}

// buildPartyTaskStatus only update status of local parties
func (h *RunningHandler) buildPartyTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trg *kusciaapisv1alpha1.TaskResourceGroup) []kusciaapisv1alpha1.PartyTaskStatus {
	partyTaskStatuses := make([]kusciaapisv1alpha1.PartyTaskStatus, 0)

	for _, party := range trg.Spec.OutOfControlledParties {
		outerPartyTaskStatus := buildOuterPartyTaskStatus(taskStatus, &party)
		partyTaskStatuses = append(partyTaskStatuses, *outerPartyTaskStatus)
	}

	for _, party := range trg.Spec.Parties {
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
	condExist := false
	switch status.Phase {
	case kusciaapisv1alpha1.TaskRunning:
		cond, condExist = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondRunning, true)
		cond.Status = v1.ConditionTrue
	case kusciaapisv1alpha1.TaskFailed:
		cond, condExist = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondSuccess, true)
		cond.Status = v1.ConditionFalse
	case kusciaapisv1alpha1.TaskSucceeded:
		cond, condExist = utilsres.GetKusciaTaskCondition(status, kusciaapisv1alpha1.KusciaTaskCondSuccess, true)
		cond.Status = v1.ConditionTrue
	}

	if !condExist {
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
	expiredCond, found := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourceGroupExpired)
	if found && expiredCond.Status == v1.ConditionTrue {
		return fmt.Sprintf("The task was not scheduled within %v seconds of its entire lifecycle", trg.Spec.LifecycleSeconds)
	}

	condReason := ""
	for _, cond := range trg.Status.Conditions {
		if cond.Status != v1.ConditionTrue && cond.Reason != "" {
			condReason += fmt.Sprintf("%v failed: %v,", cond.Type, cond.Reason)
		}
	}
	return strings.TrimSuffix(condReason, ",")
}

func buildOuterPartyTaskStatus(taskStatus *kusciaapisv1alpha1.KusciaTaskStatus, trgParty *kusciaapisv1alpha1.TaskResourceGroupParty) *kusciaapisv1alpha1.PartyTaskStatus {
	outerPartyTaskStatus := &kusciaapisv1alpha1.PartyTaskStatus{
		DomainID: trgParty.DomainID,
		Role:     trgParty.Role,
		Phase:    kusciaapisv1alpha1.TaskPending,
	}

	for _, s := range taskStatus.PartyTaskStatus {
		if s.DomainID == outerPartyTaskStatus.DomainID && s.Role == outerPartyTaskStatus.Role {
			outerPartyTaskStatus.Phase = s.Phase
			outerPartyTaskStatus.Message = s.Message
			break
		}
	}
	return outerPartyTaskStatus
}
