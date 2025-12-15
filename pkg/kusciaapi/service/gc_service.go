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

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/controllers/garbagecollection"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type IGCService interface {
	TriggerGC(ctx context.Context, request *TriggerGCRequest) (*TriggerGCResponse, error)
	UpdateGCConfig(ctx context.Context, request *UpdateGCConfigRequest) (*UpdateGCConfigResponse, error)
	QueryGCConfig(ctx context.Context, request *QueryGCConfigRequest) (*QueryGCConfigResponse, error)
	QueryGCStatus(ctx context.Context, request *QueryGCStatusRequest) (*QueryGCStatusResponse, error)
}

type gcService struct {
	configManager  *garbagecollection.GCConfigManager
	triggerManager *garbagecollection.GCTriggerManager
}

// NewGCService 创建 GC Service
func NewGCService(
	configManager *garbagecollection.GCConfigManager,
	triggerManager *garbagecollection.GCTriggerManager,
) IGCService {
	return &gcService{
		configManager:  configManager,
		triggerManager: triggerManager,
	}
}

// TriggerGC 触发 GC
func (s *gcService) TriggerGC(ctx context.Context, request *TriggerGCRequest) (*TriggerGCResponse, error) {
	if request.Async {
		// 异步执行
		go func() {
			result, err := s.triggerManager.TriggerManual(context.Background())
			if err != nil {
				nlog.Errorf("Async GC failed: %v", err)
			} else {
				nlog.Infof("Async GC completed: %+v", result)
			}
		}()

		return &TriggerGCResponse{
			Status: Status{
				Code:    0,
				Message: "GC triggered asynchronously",
			},
		}, nil
	}

	// 同步执行
	result, err := s.triggerManager.TriggerManual(ctx)
	if err != nil {
		return &TriggerGCResponse{
			Status: Status{
				Code:    -1,
				Message: fmt.Sprintf("GC failed: %v", err),
			},
		}, nil
	}

	return &TriggerGCResponse{
		Status: Status{
			Code:    0,
			Message: "GC completed successfully",
		},
		Result: convertGCResult(result),
	}, nil
}

// UpdateGCConfig 更新 GC 配置
func (s *gcService) UpdateGCConfig(ctx context.Context, request *UpdateGCConfigRequest) (*UpdateGCConfigResponse, error) {
	// 转换为内部配置结构
	config := &garbagecollection.GCConfig{
		KusciaJobGC: garbagecollection.KusciaJobGCConfig{
			DurationHours: request.Config.KusciaJobGC.DurationHours,
			BatchSize:     request.Config.KusciaJobGC.BatchSize,
			BatchInterval: request.Config.KusciaJobGC.BatchInterval,
		},
	}

	// 更新配置
	err := s.configManager.UpdateConfig(ctx, config)
	if err != nil {
		return &UpdateGCConfigResponse{
			Status: Status{
				Code:    -1,
				Message: fmt.Sprintf("failed to update config: %v", err),
			},
		}, nil
	}

	return &UpdateGCConfigResponse{
		Status: Status{
			Code:    0,
			Message: "Config updated successfully",
		},
	}, nil
}

// QueryGCConfig 查询 GC 配置
func (s *gcService) QueryGCConfig(ctx context.Context, request *QueryGCConfigRequest) (*QueryGCConfigResponse, error) {
	config := s.configManager.GetCurrentConfig()

	return &QueryGCConfigResponse{
		Status: Status{
			Code:    0,
			Message: "Success",
		},
		Config: GCConfig{
			KusciaJobGC: KusciaJobGCConfig{
				DurationHours: config.KusciaJobGC.DurationHours,
				BatchSize:     config.KusciaJobGC.BatchSize,
				BatchInterval: config.KusciaJobGC.BatchInterval,
			},
		},
	}, nil
}

// QueryGCStatus 查询 GC 状态
func (s *gcService) QueryGCStatus(ctx context.Context, request *QueryGCStatusRequest) (*QueryGCStatusResponse, error) {
	status := s.triggerManager.GetStatus()

	gcStatus := GCStatus{
		IsRunning:     status.IsRunning,
		TotalRuns:     status.TotalRuns,
		ManualRuns:    status.ManualRuns,
		ScheduledRuns: status.ScheduledRuns,
	}

	if !status.LastRunTime.IsZero() {
		gcStatus.LastRunTime = status.LastRunTime
	}

	if status.LastRunResult != nil {
		gcStatus.LastRunResult = convertGCResult(status.LastRunResult)
	}

	return &QueryGCStatusResponse{
		Status: Status{
			Code:    0,
			Message: "Success",
		},
		GCStatus: gcStatus,
	}, nil
}

// convertGCResult 转换 GC 结果
func convertGCResult(result *garbagecollection.GCResult) *GCResult {
	if result == nil {
		return nil
	}

	return &GCResult{
		ControllerName: result.ControllerName,
		DeletedCount:   result.DeletedCount,
		ErrorCount:     result.ErrorCount,
		Duration:       result.Duration.String(),
		StartTime:      result.StartTime,
		EndTime:        result.EndTime,
	}
}
