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

//nolint:dupl
package handler

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	ktcommon "github.com/secretflow/kuscia/pkg/controllers/kusciatask/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// FinishedHandler is used to handle finished kuscia task.
type FinishedHandler struct {
	kubeClient      kubernetes.Interface
	kusciaClient    kusciaclientset.Interface
	podsLister      corelisters.PodLister
	configMapLister corelisters.ConfigMapLister
}

// NewFinishedHandler returns a FinishedHandler instance.
func NewFinishedHandler(deps *ktcommon.Dependencies) *FinishedHandler {
	return &FinishedHandler{
		kubeClient:      deps.KubeClient,
		kusciaClient:    deps.KusciaClient,
		podsLister:      deps.PodsLister,
		configMapLister: deps.ConfigMapLister,
	}
}

// Handle is used to perform the real logic.
func (h *FinishedHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	if err := h.DeleteTaskResources(kusciaTask); err != nil {
		return false, err
	}

	now := metav1.Now().Rfc3339Copy()
	kusciaTask.Status.CompletionTime = &now
	return true, nil
}

// DeleteTaskResources is used to delete task resources.
func (h *FinishedHandler) DeleteTaskResources(kusciaTask *kusciaapisv1alpha1.KusciaTask) error {
	pods, _ := h.podsLister.List(labels.SelectorFromSet(labels.Set{common.LabelTaskUID: string(kusciaTask.UID)}))
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodFailed {
			continue
		}

		ns := pod.Namespace
		name := pod.Name
		e := h.kubeClient.CoreV1().Pods(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
		if e != nil {
			if k8serrors.IsNotFound(e) {
				continue
			}
			return fmt.Errorf("failed to delete pod %v/%v, %v", ns, name, e)
		}

		nlog.Debugf("Delete the pod %v/%v belonging to kusciaTask %v successfully", ns, name, kusciaTask.Name)
	}

	if kusciaTask.Status.Phase != kusciaapisv1alpha1.TaskSucceeded {
		return nil
	}

	configMaps, _ := h.configMapLister.List(labels.SelectorFromSet(labels.Set{common.LabelTaskUID: string(kusciaTask.UID)}))
	for _, configMap := range configMaps {
		ns := configMap.Namespace
		name := configMap.Name
		err := h.kubeClient.CoreV1().ConfigMaps(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to delete configmap %v/%v, %v", ns, name, err)
		}
		nlog.Debugf("Delete the configmap %v/%v belonging to kusciaTask %v successfully", ns, name, kusciaTask.Name)
	}

	if err := h.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Delete(context.Background(),
		kusciaTask.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete taskResourceGroup %v, %v", kusciaTask.Name, err)
	}

	return nil
}
