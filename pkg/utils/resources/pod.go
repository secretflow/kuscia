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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
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

	if string(patchBytes) == "{}" {
		return nil
	}

	patchFn := func() error {
		_, err = kubeClient.CoreV1().Pods(newPod.Namespace).Patch(ctx, newPod.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
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

// ExtractPodAnnotationsAndLabels is used to extract pod annotations and labels.
func ExtractPodAnnotationsAndLabels(p *corev1.Pod) *corev1.Pod {
	pp := &corev1.Pod{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations
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
