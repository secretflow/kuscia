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

//nolint:dupl
package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

// configHandler GRPC Handler
type configHandler struct {
	configService service.IConfigService
	confmanager.UnimplementedConfigServiceServer
}

func NewConfigHandler(confService service.IConfigService) confmanager.ConfigServiceServer {
	return &configHandler{
		configService: confService,
	}
}

func (h *configHandler) CreateConfig(ctx context.Context, request *confmanager.CreateConfigRequest) (*confmanager.CreateConfigResponse, error) {
	return h.configService.CreateConfig(ctx, request), nil
}

func (h *configHandler) QueryConfig(ctx context.Context, request *confmanager.QueryConfigRequest) (*confmanager.QueryConfigResponse, error) {
	return h.configService.QueryConfig(ctx, request), nil
}

func (h *configHandler) UpdateConfig(ctx context.Context, request *confmanager.UpdateConfigRequest) (*confmanager.UpdateConfigResponse, error) {
	return h.configService.UpdateConfig(ctx, request), nil
}

func (h *configHandler) DeleteConfig(ctx context.Context, request *confmanager.DeleteConfigRequest) (*confmanager.DeleteConfigResponse, error) {
	return h.configService.DeleteConfig(ctx, request), nil
}

func (h *configHandler) BatchQueryConfig(ctx context.Context, request *confmanager.BatchQueryConfigRequest) (*confmanager.BatchQueryConfigResponse, error) {
	return h.configService.BatchQueryConfig(ctx, request), nil
}
