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
