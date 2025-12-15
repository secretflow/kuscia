// Copyright 2024 Ant Group Co., Ltd.
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

package garbagecollection

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	DefaultConfigMapName      = "kuscia-gc-config"
	DefaultConfigMapNamespace = "kuscia-system"
)

// InitGCConfigMap 初始化 GC ConfigMap
func InitGCConfigMap(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	configMapName := DefaultConfigMapName

	// 检查是否已存在
	_, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("ConfigMap %s/%s already exists", namespace, configMapName)
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	// 创建默认配置
	defaultConfig := getDefaultConfig()
	configData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "kuscia",
				"type": "gc-config",
			},
		},
		Data: map[string]string{
			GCConfigKey: string(configData),
		},
	}

	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	klog.Infof("Created ConfigMap %s/%s with default GC config", namespace, configMapName)
	return nil
}
