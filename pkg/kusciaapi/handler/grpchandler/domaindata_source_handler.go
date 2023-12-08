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

//nolint:dulp
package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type domainDataSourceHandler struct {
	domainDataSourceService service.IDomainDataSourceService
	kusciaapi.UnimplementedDomainDataSourceServiceServer
}

func NewDomainDataSourceHandler(domainDataSourceService service.IDomainDataSourceService) kusciaapi.DomainDataSourceServiceServer {
	return &domainDataSourceHandler{
		domainDataSourceService: domainDataSourceService,
	}
}

func (h *domainDataSourceHandler) CreateDomainDataSource(ctx context.Context, request *kusciaapi.CreateDomainDataSourceRequest) (*kusciaapi.CreateDomainDataSourceResponse, error) {
	return h.domainDataSourceService.CreateDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) UpdateDomainDataSource(ctx context.Context, request *kusciaapi.UpdateDomainDataSourceRequest) (*kusciaapi.UpdateDomainDataSourceResponse, error) {
	return h.domainDataSourceService.UpdateDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) DeleteDomainDataSource(ctx context.Context, request *kusciaapi.DeleteDomainDataSourceRequest) (*kusciaapi.DeleteDomainDataSourceResponse, error) {
	return h.domainDataSourceService.DeleteDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) QueryDomainDataSource(ctx context.Context, request *kusciaapi.QueryDomainDataSourceRequest) (*kusciaapi.QueryDomainDataSourceResponse, error) {
	return h.domainDataSourceService.QueryDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) BatchQueryDomainDataSource(ctx context.Context, request *kusciaapi.BatchQueryDomainDataSourceRequest) (*kusciaapi.BatchQueryDomainDataSourceResponse, error) {
	return h.domainDataSourceService.BatchQueryDomainDataSource(ctx, request), nil
}
