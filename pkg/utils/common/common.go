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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

// PatchPod is used to patch pod.
func PatchPod(ctx context.Context, kubeClient kubernetes.Interface, oldPod, newPod *corev1.Pod) error {
	oldData, err := json.Marshal(oldPod)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newPod)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &corev1.Pod{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for pod %v/%v, %v", newPod.Namespace, newPod.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kubeClient.CoreV1().Pods(newPod.Namespace).Patch(ctx, newPod.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// PatchPodStatus is used to patch pod status.
func PatchPodStatus(ctx context.Context, kubeClient kubernetes.Interface, oldPod, newPod *corev1.Pod) error {
	oldData, err := json.Marshal(oldPod)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newPod)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &corev1.Pod{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for pod %v/%v, %v", newPod.Namespace, newPod.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kubeClient.CoreV1().Pods(newPod.Namespace).Patch(ctx, newPod.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// PatchService is used to patch service.
func PatchService(ctx context.Context, kubeClient kubernetes.Interface, oldSvc, newSvc *corev1.Service) error {
	oldData, err := json.Marshal(oldSvc)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newSvc)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &corev1.Service{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for service %v/%v, %v", newSvc.Namespace, newSvc.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kubeClient.CoreV1().Services(newSvc.Namespace).Patch(ctx, newSvc.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// PatchConfigMap is used to patch configmap.
func PatchConfigMap(ctx context.Context, kubeClient kubernetes.Interface, oldCm, newCm *corev1.ConfigMap) error {
	oldData, err := json.Marshal(oldCm)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newCm)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for configmap %v/%v, %v", newCm.Namespace, newCm.Name, err)
	}

	if "{}" == string(patchBytes) {
		return nil
	}

	patchFn := func() error {
		_, err = kubeClient.CoreV1().ConfigMaps(newCm.Namespace).Patch(ctx, newCm.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

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

// GetTaskResourceCondition is used to get task resource condition.
func GetTaskResourceCondition(trStatus *kusciaapisv1alpha1.TaskResourceStatus, condType kusciaapisv1alpha1.TaskResourceConditionType) *kusciaapisv1alpha1.TaskResourceCondition {
	for i, cond := range trStatus.Conditions {
		if cond.Type == condType {
			return &trStatus.Conditions[i]
		}
	}

	trStatus.Conditions = append(trStatus.Conditions, kusciaapisv1alpha1.TaskResourceCondition{Type: condType})
	return &trStatus.Conditions[len(trStatus.Conditions)-1]
}

// ExtractPodAnnotations is used to extract pod annotations.
func ExtractPodAnnotations(p *corev1.Pod) *corev1.Pod {
	pp := &corev1.Pod{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Annotations = p.Annotations
	return pp
}

// ExtractPodLabels is used to extract pod labels.
func ExtractPodLabels(p *corev1.Pod) *corev1.Pod {
	pp := &corev1.Pod{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	return pp
}

// ExtractPodSpec is used to extract pod spec.
func ExtractPodSpec(p *corev1.Pod) *corev1.Pod {
	automountServiceAccountToken := false
	pp := &corev1.Pod{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations
	pp.Spec.Containers = p.Spec.Containers
	pp.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	pp.Spec.SchedulerName = p.Spec.SchedulerName
	pp.Spec.Affinity = p.Spec.Affinity
	pp.Spec.InitContainers = p.Spec.InitContainers
	pp.Spec.NodeSelector = p.Spec.NodeSelector
	pp.Spec.ImagePullSecrets = p.Spec.ImagePullSecrets
	pp.Spec.RestartPolicy = p.Spec.RestartPolicy
	pp.Spec.Volumes = p.Spec.Volumes
	pp.Spec.Tolerations = p.Spec.Tolerations
	return pp
}

// ExtractPodStatus is used to extract pod status.
func ExtractPodStatus(p *corev1.Pod) *corev1.Pod {
	pp := &corev1.Pod{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Status.Conditions = p.Status.Conditions
	pp.Status.Message = p.Status.Message
	pp.Status.Phase = p.Status.Phase
	pp.Status.Reason = p.Status.Reason
	pp.Status.ContainerStatuses = p.Status.ContainerStatuses
	pp.Status.InitContainerStatuses = p.Status.InitContainerStatuses
	pp.Status.StartTime = p.Status.StartTime
	return pp
}

// ExtractService is used to extract service.
func ExtractService(p *corev1.Service) *corev1.Service {
	pp := &corev1.Service{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations
	pp.Spec.Type = p.Spec.Type
	pp.Spec.Selector = p.Spec.Selector
	pp.Spec.SessionAffinity = p.Spec.SessionAffinity
	pp.Spec.SessionAffinityConfig = p.Spec.SessionAffinityConfig
	pp.Spec.ExternalTrafficPolicy = p.Spec.ExternalTrafficPolicy
	pp.Spec.Ports = p.Spec.Ports
	for i := range pp.Spec.Ports {
		if pp.Spec.Ports[i].NodePort != 0 {
			pp.Spec.Ports[i].NodePort = 0
		}
	}
	return pp
}

// ExtractConfigMap is used to extract configmap.
func ExtractConfigMap(p *corev1.ConfigMap) *corev1.ConfigMap {
	pp := &corev1.ConfigMap{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations
	pp.Data = p.Data
	pp.BinaryData = p.BinaryData
	return pp
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

// CompareResourceVersion is used to compare resource version.
func CompareResourceVersion(rv1, rv2 string) bool {
	irv1, _ := strconv.Atoi(rv1)
	irv2, _ := strconv.Atoi(rv2)
	return irv1 > irv2
}
