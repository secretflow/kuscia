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
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	GCControllerName  = "kuscia-job-gc-controller"
	batchSize         = 100
	defaultGCDuration = 30 * 24 * time.Hour
)

type KusciaJobGCController struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaJobLister       kuscialistersv1alpha1.KusciaJobLister
	kusciaTaskSynced      cache.InformerSynced
	kusciaJobSynced       cache.InformerSynced
	namespaceSynced       cache.InformerSynced

	// 动态配置字段
	mu                  sync.RWMutex
	kusciaJobGCDuration time.Duration
	gcBatchSize         int
	gcBatchInterval     time.Duration

	// 触发管理器
	triggerManager *GCTriggerManager

	// 配置管理器
	configManager *GCConfigManager
}

func NewKusciaJobGCController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	// Use configured duration, if 0 use default 720 hours (30 days)
	gcDuration := defaultGCDuration
	if config.KusciaJobGCDurationHours > 0 {
		gcDuration = time.Duration(config.KusciaJobGCDurationHours) * time.Hour
	}

	gcController := &KusciaJobGCController{
		kusciaClient:          kusciaClient,
		kusciaInformerFactory: kusciaInformerFactory,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaJobLister:       kusciaJobInformer.Lister(),
		kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
		kusciaJobSynced:       kusciaJobInformer.Informer().HasSynced,
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		kusciaJobGCDuration:   gcDuration,
		gcBatchSize:           batchSize,
		gcBatchInterval:       5 * time.Second,
	}
	gcController.ctx, gcController.cancel = context.WithCancel(ctx)

	// 创建触发管理器
	gcController.triggerManager = NewGCTriggerManager(gcController)

	// 注册到全局 TriggerManager
	SetGlobalKusciaJobGCTriggerManager(gcController.triggerManager)

	// 如果提供了 GCConfigManager,设置它
	if config.GCConfigManager != nil {
		if cm, ok := config.GCConfigManager.(*GCConfigManager); ok {
			gcController.SetConfigManager(cm)
		}
	}

	return gcController
}

// SetConfigManager 设置配置管理器
func (kgc *KusciaJobGCController) SetConfigManager(cm *GCConfigManager) {
	kgc.configManager = cm
	// 注册配置更新回调
	cm.RegisterUpdateCallback(kgc.OnConfigUpdate)
}

// GetTriggerManager 获取触发管理器
func (kgc *KusciaJobGCController) GetTriggerManager() *GCTriggerManager {
	return kgc.triggerManager
}

func (kgc *KusciaJobGCController) Run(flag int) error {
	nlog.Info("Starting KusciaJobGC controller")
	kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
	kgc.kubeInformerFactory.Start(kgc.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for `%s`", kgc.Name())
	if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.namespaceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting GC workers")
	if flag == 1 {
		kgc.GarbageCollectKusciaJob(kgc.ctx, time.NewTicker(2*time.Second))
	} else {
		kgc.GarbageCollectKusciaJob(kgc.ctx, nil)
	}
	return nil
}

func (kgc *KusciaJobGCController) Stop() {
	if kgc.cancel != nil {
		kgc.cancel()
		kgc.cancel = nil
	}
}

func (kgc *KusciaJobGCController) Name() string {
	return GCControllerName
}

func (kgc *KusciaJobGCController) GarbageCollectKusciaJob(ctx context.Context, ticker *time.Ticker) {
	kgc.mu.RLock()
	gcDuration := kgc.kusciaJobGCDuration
	kgc.mu.RUnlock()

	nlog.Infof("KusciaJob GC Duration is %v", gcDuration)
	if ticker == nil {
		ticker = time.NewTicker(10 * time.Minute)
	}
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 通过 TriggerManager 执行定时清理
			_, err := kgc.triggerManager.TriggerScheduled(ctx)
			if err != nil {
				nlog.Errorf("Scheduled GC failed: %v", err)
			}
		}
	}
}

// RunOnce 执行一次清理 (实现 GarbageCollector 接口)
func (kgc *KusciaJobGCController) RunOnce(ctx context.Context) (*GCResult, error) {
	startTime := time.Now()
	result := &GCResult{
		ControllerName: kgc.Name(),
		StartTime:      startTime,
	}

	// 读取当前配置
	kgc.mu.RLock()
	gcDuration := kgc.kusciaJobGCDuration
	batchSize := kgc.gcBatchSize
	batchInterval := kgc.gcBatchInterval
	kgc.mu.RUnlock()

	nlog.Infof("Starting KusciaJob GC: gcDuration=%v, batchSize=%d, batchInterval=%v",
		gcDuration, batchSize, batchInterval)

	// 列出所有 KusciaJob
	kusciaJobs, err := kgc.kusciaJobLister.KusciaJobs(common.KusciaCrossDomain).List(labels.Everything())
	if err != nil {
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to list KusciaJobs: %v", err)
	}

	nlog.Infof("Found %d KusciaJobs in namespace %s", len(kusciaJobs), common.KusciaCrossDomain)

	deletedCount := 0
	errorCount := 0
	kusciaJobClient := kgc.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain)

	for i, kusciaJob := range kusciaJobs {
		// 检查是否已完成且超过保留期
		if kusciaJob.Status.CompletionTime != nil {
			kusciaJobTime := kusciaJob.Status.CompletionTime.Time
			durationTime := time.Since(kusciaJobTime)
			if durationTime >= gcDuration {
				nlog.Infof("Deleting KusciaJob %s (completed %v ago)",
					kusciaJob.Name, durationTime)

				err := kusciaJobClient.Delete(ctx, kusciaJob.Name, metav1.DeleteOptions{})
				if err != nil {
					nlog.Errorf("Failed to delete KusciaJob %s: %v", kusciaJob.Name, err)
					errorCount++
				} else {
					deletedCount++
				}
			}
		}

		// 批处理间隔
		if (i+1)%batchSize == 0 && batchInterval > 0 {
			nlog.Infof("Processed %d jobs, sleeping %v", i+1, batchInterval)
			time.Sleep(batchInterval)
		}
	}

	result.DeletedCount = deletedCount
	result.ErrorCount = errorCount
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	nlog.Infof("KusciaJob GC completed: deleted=%d, errors=%d, duration=%v",
		deletedCount, errorCount, result.Duration)

	return result, nil
}

// OnConfigUpdate 配置更新回调
func (kgc *KusciaJobGCController) OnConfigUpdate(oldConfig, newConfig *GCConfig) error {
	kgc.mu.Lock()
	defer kgc.mu.Unlock()

	oldDuration := kgc.kusciaJobGCDuration
	oldBatchSize := kgc.gcBatchSize
	oldBatchInterval := kgc.gcBatchInterval

	kgc.kusciaJobGCDuration = time.Duration(newConfig.KusciaJobGC.DurationHours) * time.Hour
	kgc.gcBatchSize = newConfig.KusciaJobGC.BatchSize
	kgc.gcBatchInterval = time.Duration(newConfig.KusciaJobGC.BatchInterval) * time.Second

	nlog.Infof("KusciaJob GC config updated: duration=%v->%v, batchSize=%d->%d, batchInterval=%v->%v",
		oldDuration, kgc.kusciaJobGCDuration,
		oldBatchSize, kgc.gcBatchSize,
		oldBatchInterval, kgc.gcBatchInterval)

	return nil
}
