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

// GetKusciaJobCondition gets kuscia job condition.
func GetKusciaJobCondition(status *kusciaapisv1alpha1.KusciaJobStatus,
	condType kusciaapisv1alpha1.KusciaJobConditionType,
	generateCond bool) (*kusciaapisv1alpha1.KusciaJobCondition, bool) {
	for i, cond := range status.Conditions {
		if cond.Type == condType {
			return &status.Conditions[i], true
		}
	}

	if generateCond {
		status.Conditions = append(status.Conditions, kusciaapisv1alpha1.KusciaJobCondition{Type: condType, Status: corev1.ConditionFalse})
		return &status.Conditions[len(status.Conditions)-1], false
	}
	return nil, false
}

// SetKusciaJobCondition sets kuscia job condition.
func SetKusciaJobCondition(now metav1.Time, cond *kusciaapisv1alpha1.KusciaJobCondition, status corev1.ConditionStatus, reason, message string) {
	cond.Status = status
	cond.Reason = reason
	cond.Message = message
	cond.LastTransitionTime = &now
}

// UpdateKusciaJobStage updates kuscia job stage.
func UpdateKusciaJobStage(kusciaClient kusciaclientset.Interface,
	kj *kusciaapisv1alpha1.KusciaJob,
	jobStage kusciaapisv1alpha1.JobStage,
	retries int) error {

	update := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
		if kusciaJob.Labels == nil {
			kusciaJob.Labels = make(map[string]string)
		}
		kusciaJob.Labels[common.LabelJobStage] = string(jobStage)
	}

	hasUpdated := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
		if kusciaJob.Labels == nil {
			return false
		}

		return kusciaJob.Labels[common.LabelJobStage] == string(jobStage)
	}

	update(kj)

	return UpdateKusciaJob(kusciaClient, kj, hasUpdated, update, retries)
}

// UpdateKusciaJob updates kuscia job.
func UpdateKusciaJob(kusciaClient kusciaclientset.Interface,
	kj *kusciaapisv1alpha1.KusciaJob,
	hasUpdated func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool,
	update func(kusciaJob *kusciaapisv1alpha1.KusciaJob),
	retries int) (err error) {
	kjName := kj.Name
	for i, kj := 0, kj; ; i++ {
		nlog.Infof("update kuscia job %v", kjName)
		_, err = kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Update(context.Background(), kj, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}

		nlog.Warnf("Failed to update kuscia job %v spec, %v", kjName, err)
		if i >= retries {
			break
		}

		kj, err = kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Get(context.Background(), kjName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("update kuscia job %v spec failed, %v", kjName, err)
		}

		if hasUpdated(kj) {
			return nil
		}
		update(kj)
	}
	return err
}

// UpdateKusciaJobStatus will update kuscia job status. The function will retry updating if failed.
func UpdateKusciaJobStatus(kusciaClient kusciaclientset.Interface, rawKj, curKj *kusciaapisv1alpha1.KusciaJob) (err error) {
	if reflect.DeepEqual(rawKj.Status, curKj.Status) {
		nlog.Debugf("Kuscia job %v status does not change, skip to update it", curKj.Name)
		return nil
	}
	nlog.Infof("Start updating kuscia job %q status", rawKj.Name)
	if _, err = kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).UpdateStatus(context.Background(), curKj, metav1.UpdateOptions{}); err != nil && !k8serrors.IsConflict(err) {
		return err
	}
	nlog.Infof("Finish updating kuscia job %q status", rawKj.Name)
	return nil
}

// MergeKusciaJobConditions merges kuscia job conditions.
func MergeKusciaJobConditions(kusciaJob *kusciaapisv1alpha1.KusciaJob, conds []kusciaapisv1alpha1.KusciaJobCondition) {
	var addedCond []kusciaapisv1alpha1.KusciaJobCondition
	for _, cond := range conds {
		found := false
		for curIdx, curCond := range kusciaJob.Status.Conditions {
			if curCond.Type == cond.Type {
				found = true
				if !reflect.DeepEqual(cond, curCond) {
					kusciaJob.Status.Conditions[curIdx] = *cond.DeepCopy()
				}
				break
			}
		}

		if !found {
			addedCond = append(addedCond, *cond.DeepCopy())
		}
	}

	if len(addedCond) > 0 {
		kusciaJob.Status.Conditions = append(kusciaJob.Status.Conditions, addedCond...)
	}
}

// MergeKusciaJobTaskStatus merges task status of kuscia job.
func MergeKusciaJobTaskStatus(kusciaJob *kusciaapisv1alpha1.KusciaJob, taskPhase map[string]kusciaapisv1alpha1.KusciaTaskPhase) bool {
	updated := false
	if len(taskPhase) == 0 {
		return false
	}

	if kusciaJob.Status.TaskStatus == nil {
		kusciaJob.Status.TaskStatus = taskPhase
		return true
	}

	for taskID, kjTaskPhase := range kusciaJob.Status.TaskStatus {
		phase, ok := taskPhase[taskID]
		if !ok {
			continue
		}

		if phase != kjTaskPhase {
			updated = true
			kusciaJob.Status.TaskStatus[taskID] = phase
		}
	}

	var addedTaskStatus []string
	for taskID := range taskPhase {
		if _, ok := kusciaJob.Status.TaskStatus[taskID]; !ok {
			addedTaskStatus = append(addedTaskStatus, taskID)
		}
	}

	for _, taskID := range addedTaskStatus {
		updated = true
		kusciaJob.Status.TaskStatus[taskID] = taskPhase[taskID]
	}
	return updated
}
