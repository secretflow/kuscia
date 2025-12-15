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
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// ConfigMap 配置键
	GCConfigKey = "gc-config.json"

	// 默认配置值
	DefaultGCDurationHours = 720 // 30天
	DefaultBatchSize       = 100
	DefaultBatchInterval   = 5 // 秒
)

// GCConfigManager 管理 GC 配置的动态加载和更新
type GCConfigManager struct {
	kubeClient        kubernetes.Interface
	configMapInformer coreinformers.ConfigMapInformer
	configMapLister   corev1lister.ConfigMapLister
	namespace         string
	configMapName     string

	// 配置缓存
	mu            sync.RWMutex
	currentConfig *GCConfig

	// 配置更新回调
	updateCallbacks []ConfigUpdateCallback
}

// GCConfig GC 配置结构
type GCConfig struct {
	KusciaJobGC KusciaJobGCConfig `json:"kusciaJobGC"`
}

// KusciaJobGCConfig KusciaJob GC 配置
type KusciaJobGCConfig struct {
	DurationHours int `json:"durationHours"` // 保留小时数
	BatchSize     int `json:"batchSize"`     // 批处理大小
	BatchInterval int `json:"batchInterval"` // 批次间隔(秒)
}

// ConfigUpdateCallback 配置更新回调函数
type ConfigUpdateCallback func(oldConfig, newConfig *GCConfig) error

// NewGCConfigManager 创建配置管理器
func NewGCConfigManager(
	kubeClient kubernetes.Interface,
	namespace string,
	configMapName string,
) (*GCConfigManager, error) {

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		1*time.Minute,
		informers.WithNamespace(namespace),
	)

	configMapInformer := informerFactory.Core().V1().ConfigMaps()

	cm := &GCConfigManager{
		kubeClient:        kubeClient,
		configMapInformer: configMapInformer,
		configMapLister:   configMapInformer.Lister(),
		namespace:         namespace,
		configMapName:     configMapName,
		currentConfig:     getDefaultConfig(),
		updateCallbacks:   make([]ConfigUpdateCallback, 0),
	}

	return cm, nil
}

// Start 启动配置管理器
func (cm *GCConfigManager) Start(ctx context.Context) error {
	// 注册事件处理器
	cm.configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm.handleConfigMapUpdate(nil, obj)
		},
		UpdateFunc: cm.handleConfigMapUpdate,
	})

	// 启动 Informer
	go cm.configMapInformer.Informer().Run(ctx.Done())

	// 等待缓存同步
	if !cache.WaitForCacheSync(ctx.Done(), cm.configMapInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync configmap cache")
	}

	// 加载初始配置
	if err := cm.loadConfig(); err != nil {
		klog.Warningf("Failed to load initial GC config, using defaults: %v", err)
	}

	klog.Infof("GC ConfigManager started, namespace=%s, configMap=%s", cm.namespace, cm.configMapName)

	return nil
}

// handleConfigMapUpdate 处理 ConfigMap 更新事件
func (cm *GCConfigManager) handleConfigMapUpdate(oldObj, newObj interface{}) {
	newCM, ok := newObj.(*corev1.ConfigMap)
	if !ok || newCM.Name != cm.configMapName {
		return
	}

	klog.V(4).Infof("ConfigMap %s/%s updated, reloading GC config", cm.namespace, cm.configMapName)

	oldConfig := cm.GetCurrentConfig()
	newConfig := cm.parseConfigMap(newCM)

	if newConfig == nil {
		klog.Warningf("Failed to parse new config, keeping old config")
		return
	}

	// 更新缓存
	cm.mu.Lock()
	cm.currentConfig = newConfig
	cm.mu.Unlock()

	// 通知所有注册的回调
	for _, callback := range cm.updateCallbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			klog.Errorf("Failed to execute config update callback: %v", err)
		}
	}

	klog.Infof("GC config updated successfully: %+v", newConfig)
}

// loadConfig 加载配置
func (cm *GCConfigManager) loadConfig() error {
	configMap, err := cm.configMapLister.ConfigMaps(cm.namespace).Get(cm.configMapName)
	if err != nil {
		return err
	}

	newConfig := cm.parseConfigMap(configMap)
	if newConfig == nil {
		return fmt.Errorf("failed to parse configmap")
	}

	cm.mu.Lock()
	cm.currentConfig = newConfig
	cm.mu.Unlock()

	return nil
}

// parseConfigMap 解析 ConfigMap
func (cm *GCConfigManager) parseConfigMap(configMap *corev1.ConfigMap) *GCConfig {
	if configMap == nil || configMap.Data == nil {
		return nil
	}

	configData, exists := configMap.Data[GCConfigKey]
	if !exists {
		klog.Warningf("ConfigMap %s/%s missing key %s", cm.namespace, cm.configMapName, GCConfigKey)
		return nil
	}

	var config GCConfig
	if err := json.Unmarshal([]byte(configData), &config); err != nil {
		klog.Errorf("Failed to unmarshal GC config: %v", err)
		return nil
	}

	// 配置验证和默认值设置
	if config.KusciaJobGC.DurationHours <= 0 {
		config.KusciaJobGC.DurationHours = DefaultGCDurationHours
	}
	if config.KusciaJobGC.BatchSize <= 0 {
		config.KusciaJobGC.BatchSize = DefaultBatchSize
	}
	if config.KusciaJobGC.BatchInterval < 0 {
		config.KusciaJobGC.BatchInterval = DefaultBatchInterval
	}

	return &config
}

// GetCurrentConfig 获取当前配置(线程安全)
func (cm *GCConfigManager) GetCurrentConfig() *GCConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回副本
	configCopy := *cm.currentConfig
	return &configCopy
}

// UpdateConfig 更新配置到 ConfigMap
func (cm *GCConfigManager) UpdateConfig(ctx context.Context, newConfig *GCConfig) error {
	// 配置验证
	if newConfig.KusciaJobGC.DurationHours < 1 {
		return fmt.Errorf("durationHours must be at least 1 hour")
	}
	if newConfig.KusciaJobGC.BatchSize < 1 || newConfig.KusciaJobGC.BatchSize > 1000 {
		return fmt.Errorf("batchSize must be between 1 and 1000")
	}
	if newConfig.KusciaJobGC.BatchInterval < 0 || newConfig.KusciaJobGC.BatchInterval > 60 {
		return fmt.Errorf("batchInterval must be between 0 and 60 seconds")
	}

	// 序列化配置
	configData, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// 获取现有 ConfigMap
	configMap, err := cm.kubeClient.CoreV1().ConfigMaps(cm.namespace).Get(ctx, cm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get configmap: %v", err)
	}

	// 更新配置
	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}
	configMap.Data[GCConfigKey] = string(configData)

	// 更新到 K8s
	_, err = cm.kubeClient.CoreV1().ConfigMaps(cm.namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update configmap: %v", err)
	}

	klog.Infof("ConfigMap %s/%s updated successfully", cm.namespace, cm.configMapName)

	return nil
}

// RegisterUpdateCallback 注册配置更新回调
func (cm *GCConfigManager) RegisterUpdateCallback(callback ConfigUpdateCallback) {
	cm.updateCallbacks = append(cm.updateCallbacks, callback)
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *GCConfig {
	return &GCConfig{
		KusciaJobGC: KusciaJobGCConfig{
			DurationHours: DefaultGCDurationHours,
			BatchSize:     DefaultBatchSize,
			BatchInterval: DefaultBatchInterval,
		},
	}
}
