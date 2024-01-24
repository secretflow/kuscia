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

package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/confmanager/errorcode"
	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

// configurationHandler GRPC Handler
type configurationHandler struct {
	configurationService service.IConfigurationService
	confmanager.UnimplementedConfigurationServiceServer
}

func NewConfigurationHandler(confService service.IConfigurationService) confmanager.ConfigurationServiceServer {
	return &configurationHandler{
		configurationService: confService,
	}
}

func (h *configurationHandler) CreateConfiguration(ctx context.Context, request *confmanager.CreateConfigurationRequest) (*confmanager.CreateConfigurationResponse, error) {
	tlsCert := interceptor.TLSCertFromGRPCContext(ctx)
	if tlsCert == nil || len(tlsCert.OrganizationalUnit) == 0 {
		return &confmanager.CreateConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "require client tls ou"),
		}, nil
	}
	response := h.configurationService.CreateConfiguration(ctx, request, tlsCert.OrganizationalUnit[0])
	return &response, nil
}

func (h *configurationHandler) QueryConfiguration(ctx context.Context, request *confmanager.QueryConfigurationRequest) (*confmanager.QueryConfigurationResponse, error) {
	tlsCert := interceptor.TLSCertFromGRPCContext(ctx)
	if tlsCert == nil || len(tlsCert.OrganizationalUnit) == 0 {
		return &confmanager.QueryConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "require client tls ou"),
		}, nil
	}
	response := h.configurationService.QueryConfiguration(ctx, request, tlsCert.OrganizationalUnit[0])
	return &response, nil
}
