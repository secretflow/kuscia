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
	"sync/atomic"
	"time"

	"k8s.io/klog/v2"
)

var (
	// 全局的 KusciaJob GC TriggerManager
	globalKusciaJobGCTriggerManager *GCTriggerManager
	globalTriggerManagerMu          sync.RWMutex
)

// SetGlobalKusciaJobGCTriggerManager 设置全局的 KusciaJob GC TriggerManager
func SetGlobalKusciaJobGCTriggerManager(tm *GCTriggerManager) {
	globalTriggerManagerMu.Lock()
	defer globalTriggerManagerMu.Unlock()
	globalKusciaJobGCTriggerManager = tm
}

// GetGlobalKusciaJobGCTriggerManager 获取全局的 KusciaJob GC TriggerManager
func GetGlobalKusciaJobGCTriggerManager() *GCTriggerManager {
	globalTriggerManagerMu.RLock()
	defer globalTriggerManagerMu.RUnlock()
	return globalKusciaJobGCTriggerManager
}

// GCTriggerManager 触发管理器
type GCTriggerManager struct {
	// 执行锁 - 防止并发执行
	mu        sync.Mutex
	isRunning int32 // 使用原子操作

	// 统计信息
	lastRunTime   time.Time
	lastRunResult *GCResult
	totalRuns     int64
	manualRuns    int64
	scheduledRuns int64

	// GC Controller
	gcController GarbageCollector
}

// GarbageCollector GC 控制器接口
type GarbageCollector interface {
	Name() string
	RunOnce(ctx context.Context) (*GCResult, error)
}

// GCResult GC 执行结果
type GCResult struct {
	ControllerName string        `json:"controllerName"`
	DeletedCount   int           `json:"deletedCount"`
	ErrorCount     int           `json:"errorCount"`
	Duration       time.Duration `json:"duration"`
	StartTime      time.Time     `json:"startTime"`
	EndTime        time.Time     `json:"endTime"`
}

// GCStatus GC 运行状态
type GCStatus struct {
	IsRunning     bool      `json:"isRunning"`
	LastRunTime   time.Time `json:"lastRunTime"`
	LastRunResult *GCResult `json:"lastRunResult,omitempty"`
	TotalRuns     int64     `json:"totalRuns"`
	ManualRuns    int64     `json:"manualRuns"`
	ScheduledRuns int64     `json:"scheduledRuns"`
}

// NewGCTriggerManager 创建触发管理器
func NewGCTriggerManager(gcController GarbageCollector) *GCTriggerManager {
	return &GCTriggerManager{
		gcController: gcController,
		isRunning:    0,
	}
}

// TriggerManual 手动触发清理
func (tm *GCTriggerManager) TriggerManual(ctx context.Context) (*GCResult, error) {
	// 使用原子操作检查并设置运行状态
	if !atomic.CompareAndSwapInt32(&tm.isRunning, 0, 1) {
		return nil, fmt.Errorf("GC is already running, please try again later")
	}

	defer func() {
		atomic.StoreInt32(&tm.isRunning, 0)
		atomic.AddInt64(&tm.manualRuns, 1)
		atomic.AddInt64(&tm.totalRuns, 1)
	}()

	klog.Infof("Manual GC triggered for %s", tm.gcController.Name())

	result, err := tm.gcController.RunOnce(ctx)

	tm.mu.Lock()
	tm.lastRunTime = time.Now()
	tm.lastRunResult = result
	tm.mu.Unlock()

	if err != nil {
		klog.Errorf("Manual GC failed: %v", err)
		return result, err
	}

	klog.Infof("Manual GC completed: %+v", result)
	return result, nil
}

// TriggerScheduled 定时触发清理
func (tm *GCTriggerManager) TriggerScheduled(ctx context.Context) (*GCResult, error) {
	// 定时任务不阻塞,如果正在运行则跳过
	if !atomic.CompareAndSwapInt32(&tm.isRunning, 0, 1) {
		klog.V(4).Infof("GC is already running, skip scheduled run for %s", tm.gcController.Name())
		return nil, nil
	}

	defer func() {
		atomic.StoreInt32(&tm.isRunning, 0)
		atomic.AddInt64(&tm.scheduledRuns, 1)
		atomic.AddInt64(&tm.totalRuns, 1)
	}()

	klog.V(4).Infof("Scheduled GC triggered for %s", tm.gcController.Name())

	result, err := tm.gcController.RunOnce(ctx)

	tm.mu.Lock()
	tm.lastRunTime = time.Now()
	tm.lastRunResult = result
	tm.mu.Unlock()

	if err != nil {
		klog.Errorf("Scheduled GC failed: %v", err)
		return result, err
	}

	klog.V(4).Infof("Scheduled GC completed: %+v", result)
	return result, nil
}

// GetStatus 获取运行状态
func (tm *GCTriggerManager) GetStatus() *GCStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return &GCStatus{
		IsRunning:     atomic.LoadInt32(&tm.isRunning) == 1,
		LastRunTime:   tm.lastRunTime,
		LastRunResult: tm.lastRunResult,
		TotalRuns:     atomic.LoadInt64(&tm.totalRuns),
		ManualRuns:    atomic.LoadInt64(&tm.manualRuns),
		ScheduledRuns: atomic.LoadInt64(&tm.scheduledRuns),
	}
}

// IsRunning 检查是否正在运行
func (tm *GCTriggerManager) IsRunning() bool {
	return atomic.LoadInt32(&tm.isRunning) == 1
}
