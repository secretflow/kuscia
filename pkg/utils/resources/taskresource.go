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

// PatchTaskResource is used to patch task resource.
func PatchTaskResource(ctx context.Context, kusciaClient kusciaclientset.Interface, oldTr, newTr *kusciaapisv1alpha1.TaskResource) error {
	oldData, err := json.Marshal(oldTr)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newTr)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.TaskResource{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for task resource %v/%v, %v", newTr.Namespace, newTr.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().TaskResources(newTr.Namespace).Patch(ctx, newTr.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// ExtractTaskResource is used to extract task resource.
func ExtractTaskResource(p *kusciaapisv1alpha1.TaskResource) *kusciaapisv1alpha1.TaskResource {
	pp := &kusciaapisv1alpha1.TaskResource{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations
	pp.Spec = p.Spec
	pp.Status = p.Status
	return pp
}

// ExtractTaskResourceStatus is used to extract task resource status.
func ExtractTaskResourceStatus(p *kusciaapisv1alpha1.TaskResource) *kusciaapisv1alpha1.TaskResource {
	pp := &kusciaapisv1alpha1.TaskResource{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Status = p.Status
	return pp
}

// GetTaskResourceCondition gets task resource condition.
func GetTaskResourceCondition(trStatus *kusciaapisv1alpha1.TaskResourceStatus, condType kusciaapisv1alpha1.TaskResourceConditionType) *kusciaapisv1alpha1.TaskResourceCondition {
	for i, cond := range trStatus.Conditions {
		if cond.Type == condType {
			return &trStatus.Conditions[i]
		}
	}

	trStatus.Conditions = append(trStatus.Conditions, kusciaapisv1alpha1.TaskResourceCondition{Type: condType, Status: corev1.ConditionFalse})
	return &trStatus.Conditions[len(trStatus.Conditions)-1]
}
