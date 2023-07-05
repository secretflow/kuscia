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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/util/retry"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

// PatchTaskResourceGroupStatus is used to patch task resource group status.
func PatchTaskResourceGroupStatus(ctx context.Context, kusciaClient kusciaclientset.Interface, oldTrg, newTrg *kusciaapisv1alpha1.TaskResourceGroup) error {
	oldData, err := json.Marshal(kusciaapisv1alpha1.TaskResourceGroup{Status: oldTrg.Status})
	if err != nil {
		return err
	}

	newData, err := json.Marshal(kusciaapisv1alpha1.TaskResourceGroup{Status: newTrg.Status})
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.TaskResourceGroup{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for task resouece group %v: %v", newTrg.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().TaskResourceGroups().Patch(ctx, newTrg.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// IsExistingTaskResourceGroupCondition checks if TaskResourceGroupCondition exists.
func IsExistingTaskResourceGroupCondition(status *kusciaapisv1alpha1.TaskResourceGroupStatus, condType kusciaapisv1alpha1.TaskResourceGroupConditionType, condStatus corev1.ConditionStatus) bool {
	for _, cond := range status.Conditions {
		if cond.Type == condType && cond.Status == condStatus {
			return true
		}
	}
	return false
}

// GetTaskResourceGroupCondition is used to get task resource group condition.
func GetTaskResourceGroupCondition(trgStatus *kusciaapisv1alpha1.TaskResourceGroupStatus, condType kusciaapisv1alpha1.TaskResourceGroupConditionType) (*kusciaapisv1alpha1.TaskResourceGroupCondition, bool) {
	for i, cond := range trgStatus.Conditions {
		if cond.Type == condType {
			return &trgStatus.Conditions[i], true
		}
	}

	trgStatus.Conditions = append(trgStatus.Conditions, kusciaapisv1alpha1.TaskResourceGroupCondition{Type: condType})
	return &trgStatus.Conditions[len(trgStatus.Conditions)-1], false
}

// SetTaskResourceGroupCondition sets task resource group condition.
func SetTaskResourceGroupCondition(now *metav1.Time, cond *kusciaapisv1alpha1.TaskResourceGroupCondition, condStatus corev1.ConditionStatus, condReason string) bool {
	if cond.Status != condStatus || cond.Reason != condReason {
		cond.Status = condStatus
		cond.Reason = condReason
		cond.LastTransitionTime = now
		return true
	}
	return false
}
