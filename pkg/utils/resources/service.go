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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

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

// ExtractService is used to extract service.
func ExtractService(p *corev1.Service) *corev1.Service {
	pp := &corev1.Service{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	pp.Annotations = p.Annotations

	if p.Spec.ClusterIP == "None" {
		pp.Spec.ClusterIP = "None"
	}
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

// ExtractServiceLabels is used to extract service labels.
func ExtractServiceLabels(p *corev1.Service) *corev1.Service {
	pp := &corev1.Service{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	return pp
}

// UpdateServiceAnnotations updates service annotations.
func UpdateServiceAnnotations(kubeClient kubernetes.Interface, service *corev1.Service, at map[string]string) (err error) {
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}

	for k, v := range at {
		service.Annotations[k] = v
	}
	updateFn := func() error {
		_, err = kubeClient.CoreV1().Services(service.GetNamespace()).Update(context.Background(), service, metav1.UpdateOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, updateFn)
}

// GenerateServiceName is used to generate service name.
// Service name generation rules：
// 1. If the first character is a number, add svc- as a prefix;
// 2. If the length of the name exceeds 63 characters, it will be truncated to 63 characters.
// 3. The final name must comply with DNS subdomain naming rules.
func GenerateServiceName(prefix, portName string) string {
	prefix = strings.Trim(prefix, "-")
	portName = strings.Trim(portName, "-")
	name := fmt.Sprintf("%s-%s", prefix, portName)
	if name[0] >= '0' && name[0] <= '9' {
		name = "svc-" + name
	}

	if len(name) > 63 {
		hash := sha256.Sum256([]byte(name))
		hashStr := fmt.Sprintf("%x", hash)
		maxPrefixLen := 63 - 16 - len(portName) - 6
		name = fmt.Sprintf("svc-%s-%s-%s", prefix[:maxPrefixLen], portName, hashStr[:16])
	}
	return name
}
