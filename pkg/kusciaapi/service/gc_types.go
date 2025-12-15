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

package service

import "time"

// TriggerGCRequest 触发 GC 请求
type TriggerGCRequest struct {
	Async bool `json:"async"` // 是否异步执行
}

// TriggerGCResponse 触发 GC 响应
type TriggerGCResponse struct {
	Status Status    `json:"status"`
	Result *GCResult `json:"result,omitempty"`
}

// UpdateGCConfigRequest 更新 GC 配置请求
type UpdateGCConfigRequest struct {
	Config GCConfig `json:"config"`
}

// UpdateGCConfigResponse 更新 GC 配置响应
type UpdateGCConfigResponse struct {
	Status Status `json:"status"`
}

// QueryGCConfigRequest 查询 GC 配置请求
type QueryGCConfigRequest struct {
	// 空请求
}

// QueryGCConfigResponse 查询 GC 配置响应
type QueryGCConfigResponse struct {
	Status Status   `json:"status"`
	Config GCConfig `json:"config"`
}

// QueryGCStatusRequest 查询 GC 状态请求
type QueryGCStatusRequest struct {
	// 空请求
}

// QueryGCStatusResponse 查询 GC 状态响应
type QueryGCStatusResponse struct {
	Status   Status   `json:"status"`
	GCStatus GCStatus `json:"gcStatus"`
}

// GCConfig GC 配置
type GCConfig struct {
	KusciaJobGC KusciaJobGCConfig `json:"kusciaJobGC"`
}

// KusciaJobGCConfig KusciaJob GC 配置
type KusciaJobGCConfig struct {
	DurationHours int `json:"durationHours"` // 保留小时数
	BatchSize     int `json:"batchSize"`     // 批处理大小
	BatchInterval int `json:"batchInterval"` // 批次间隔(秒)
}

// GCResult GC 执行结果
type GCResult struct {
	ControllerName string    `json:"controllerName"`
	DeletedCount   int       `json:"deletedCount"`
	ErrorCount     int       `json:"errorCount"`
	Duration       string    `json:"duration"` // 格式: "1m30s"
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
}

// GCStatus GC 运行状态
type GCStatus struct {
	IsRunning     bool      `json:"isRunning"`
	LastRunTime   time.Time `json:"lastRunTime,omitempty"`
	LastRunResult *GCResult `json:"lastRunResult,omitempty"`
	TotalRuns     int64     `json:"totalRuns"`
	ManualRuns    int64     `json:"manualRuns"`
	ScheduledRuns int64     `json:"scheduledRuns"`
}

// Status 通用状态
type Status struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
