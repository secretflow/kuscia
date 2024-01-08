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

package resource

import (
	"context"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// KubeResourceManager acts as a passthrough to a cache (lister) for pods assigned to the current node.
// It is also a passthrough to a cache (lister) for Kubernetes secrets and config maps.
type KubeResourceManager struct {
	podLister       corev1listers.PodNamespaceLister
	secretLister    corev1listers.SecretNamespaceLister
	configmapLister corev1listers.ConfigMapNamespaceLister
	kubeClient      kubernetes.Interface
	namespace       string
}

func NewResourceManager(kubeClient kubernetes.Interface,
	namespace string,
	podLister corev1listers.PodNamespaceLister,
	secretLister corev1listers.SecretNamespaceLister,
	configMapLister corev1listers.ConfigMapNamespaceLister) *KubeResourceManager {
	rm := KubeResourceManager{
		podLister:       podLister,
		kubeClient:      kubeClient,
		secretLister:    secretLister,
		configmapLister: configMapLister,
		namespace:       namespace,
	}
	return &rm
}

func (rm *KubeResourceManager) GetConfigMap(name string) (*v1.ConfigMap, error) {
	cm, err := rm.configmapLister.Get(name)
	if k8serrors.IsNotFound(err) {
		return rm.kubeClient.CoreV1().ConfigMaps(rm.namespace).Get(context.Background(), name, metav1.GetOptions{})
	}

	return cm, err
}

func (rm *KubeResourceManager) GetSecret(name string) (*v1.Secret, error) {
	secret, err := rm.secretLister.Get(name)
	if k8serrors.IsNotFound(err) {
		return rm.kubeClient.CoreV1().Secrets(rm.namespace).Get(context.Background(), name, metav1.GetOptions{})
	}

	return secret, nil
}

func (rm *KubeResourceManager) GetPod(name string) (*v1.Pod, error) {
	return rm.podLister.Get(name)
}
func (rm *KubeResourceManager) GetNode(name string) (*v1.Node, error) {
	return rm.kubeClient.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
}
