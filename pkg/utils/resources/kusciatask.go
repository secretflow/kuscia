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

package resources

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// GetKusciaTaskCondition gets kuscia task condition.
func GetKusciaTaskCondition(status *kusciaapisv1alpha1.KusciaTaskStatus, condType kusciaapisv1alpha1.KusciaTaskConditionType, generateCond bool) (*kusciaapisv1alpha1.KusciaTaskCondition, bool) {
	for i, condition := range status.Conditions {
		if condition.Type == condType {
			return &status.Conditions[i], true
		}
	}

	if generateCond {
		status.Conditions = append(status.Conditions, kusciaapisv1alpha1.KusciaTaskCondition{Type: condType, Status: corev1.ConditionFalse})
		return &status.Conditions[len(status.Conditions)-1], false
	}

	return nil, false
}

// SetKusciaTaskCondition sets kuscia task condition.
func SetKusciaTaskCondition(now metav1.Time, cond *kusciaapisv1alpha1.KusciaTaskCondition, condStatus corev1.ConditionStatus, condReason, condMessage string) bool {
	if cond.Status != condStatus || cond.Reason != condReason || cond.Message != condMessage {
		cond.Status = condStatus
		cond.Reason = condReason
		cond.Message = condMessage
		cond.LastTransitionTime = &now
		return true
	}
	return false
}

// UpdateKusciaTaskStatus updates kuscia task status.
func UpdateKusciaTaskStatus(kusciaClient kusciaclientset.Interface, rawKt, curKt *kusciaapisv1alpha1.KusciaTask) (err error) {
	if reflect.DeepEqual(rawKt.Status, curKt.Status) {
		nlog.Debugf("Kuscia task %v status does not change, skip to update it", curKt.Name)
		return nil
	}
	nlog.Infof("Start updating kuscia task %q status", rawKt.Name)
	if _, err = kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).UpdateStatus(context.Background(), curKt, metav1.UpdateOptions{}); err != nil {
		return err
	}
	nlog.Infof("Finish updating kuscia task %q status", rawKt.Name)
	return nil
}

// UpdateKusciaTaskStatusWithRetry updates kuscia task status with retry.
func UpdateKusciaTaskStatusWithRetry(kusciaClient kusciaclientset.Interface, rawKt, curKt *kusciaapisv1alpha1.KusciaTask, retryCount int) (err error) {
	if reflect.DeepEqual(rawKt.Status, curKt.Status) {
		nlog.Debugf("Kuscia task %v status does not change, skip to update it", curKt.Name)
		return nil
	}

	var lastErr error
	newStatus := curKt.Status.DeepCopy()
	for i, curKt := 0, curKt; ; i++ {
		nlog.Infof("Start updating kuscia task %q status", rawKt.Name)
		if _, err = kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).UpdateStatus(context.Background(), curKt, metav1.UpdateOptions{}); err == nil {
			nlog.Infof("Finish updating kuscia task %q status", rawKt.Name)
			return nil
		}

		if k8serrors.IsConflict(err) {
			nlog.Warnf("Failed to update kuscia task %q status since the resource version changed, skip updating it, last updating error: %v", rawKt.Name, err)
			return nil
		}

		nlog.Warnf("Failed to update kuscia task %q status, %v", rawKt.Name, err)
		lastErr = err
		if i >= retryCount {
			break
		}

		curKt, err = kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(context.Background(), rawKt.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get the newest kuscia task %q, %v", rawKt.Name, err)
		}

		if reflect.DeepEqual(curKt.Status, newStatus) {
			nlog.Infof("Kuscia task %v status is already updated, skip to update it", rawKt.Name)
			return nil
		}

		curKt.Status.Phase = newStatus.Phase
		curKt.Status.Reason = newStatus.Reason
		curKt.Status.Message = newStatus.Message
		curKt.Status.PodStatuses = newStatus.PodStatuses
		curKt.Status.StartTime = newStatus.StartTime
		curKt.Status.CompletionTime = newStatus.CompletionTime
		curKt.Status.LastReconcileTime = newStatus.LastReconcileTime
		MergeKusciaTaskConditions(curKt, newStatus.Conditions)
		MergeKusciaTaskPartyTaskStatus(curKt, newStatus.PartyTaskStatus)
	}

	return lastErr
}

// MergeKusciaTaskConditions merges kuscia task conditions.
func MergeKusciaTaskConditions(kusciaTask *kusciaapisv1alpha1.KusciaTask, conds []kusciaapisv1alpha1.KusciaTaskCondition) {
	var addedCond []kusciaapisv1alpha1.KusciaTaskCondition
	for _, cond := range conds {
		found := false
		for curIdx, curCond := range kusciaTask.Status.Conditions {
			if curCond.Type == cond.Type {
				found = true
				if !reflect.DeepEqual(cond, curCond) {
					kusciaTask.Status.Conditions[curIdx] = *cond.DeepCopy()
				}
				break
			}
		}

		if !found {
			addedCond = append(addedCond, *cond.DeepCopy())
		}
	}

	if len(addedCond) > 0 {
		kusciaTask.Status.Conditions = append(kusciaTask.Status.Conditions, addedCond...)
	}
}

// MergeKusciaTaskPartyTaskStatus merges party task status of Kuscia task.
func MergeKusciaTaskPartyTaskStatus(kusciaTask *kusciaapisv1alpha1.KusciaTask, partyTaskStatus []kusciaapisv1alpha1.PartyTaskStatus) {
	var addedPartyTaskStatus []kusciaapisv1alpha1.PartyTaskStatus
	for idx, status := range partyTaskStatus {
		found := false
		for curIdx, curStatus := range kusciaTask.Status.PartyTaskStatus {
			if status.DomainID == curStatus.DomainID && status.Role == curStatus.Role {
				found = true
				if curStatus.Phase == kusciaapisv1alpha1.TaskFailed || curStatus.Phase == kusciaapisv1alpha1.TaskSucceeded {
					break
				}

				if !reflect.DeepEqual(status, curStatus) {
					kusciaTask.Status.PartyTaskStatus[curIdx] = *status.DeepCopy()
					break
				}
			}
		}

		if !found {
			addedPartyTaskStatus = append(addedPartyTaskStatus, partyTaskStatus[idx])
		}
	}

	if len(addedPartyTaskStatus) > 0 {
		kusciaTask.Status.PartyTaskStatus = append(kusciaTask.Status.PartyTaskStatus, addedPartyTaskStatus...)
	}
}
