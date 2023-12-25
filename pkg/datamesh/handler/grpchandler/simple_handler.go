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

	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

// DomainDataSource GRPC Handler
type domainDataHandler struct {
	domainDataService service.IDomainDataService
	datamesh.UnimplementedDomainDataServiceServer
}

func NewDomainDataHandler(domainService service.IDomainDataService) datamesh.DomainDataServiceServer {
	return &domainDataHandler{
		domainDataService: domainService,
	}
}

func (h *domainDataHandler) CreateDomainData(ctx context.Context, request *datamesh.CreateDomainDataRequest) (*datamesh.CreateDomainDataResponse, error) {
	return h.domainDataService.CreateDomainData(ctx, request), nil
}

func (h *domainDataHandler) QueryDomainData(ctx context.Context, request *datamesh.QueryDomainDataRequest) (*datamesh.QueryDomainDataResponse, error) {
	return h.domainDataService.QueryDomainData(ctx, request), nil
}

func (h *domainDataHandler) UpdateDomainData(ctx context.Context, request *datamesh.UpdateDomainDataRequest) (*datamesh.UpdateDomainDataResponse, error) {
	return h.domainDataService.UpdateDomainData(ctx, request), nil
}

func (h *domainDataHandler) DeleteDomainData(ctx context.Context, request *datamesh.DeleteDomainDataRequest) (*datamesh.DeleteDomainDataResponse, error) {
	return h.domainDataService.DeleteDomainData(ctx, request), nil
}

// DomainDataSource GRPC Handler
type domainDataSourceHandler struct {
	domainDataSourceService service.IDomainDataSourceService
	datamesh.UnimplementedDomainDataSourceServiceServer
}

func NewDomainDataSourceHandler(domaindataSourceService service.IDomainDataSourceService) datamesh.DomainDataSourceServiceServer {
	return &domainDataSourceHandler{
		domainDataSourceService: domaindataSourceService,
	}
}

func (h *domainDataSourceHandler) CreateDomainDataSource(ctx context.Context, request *datamesh.CreateDomainDataSourceRequest) (*datamesh.CreateDomainDataSourceResponse, error) {
	return h.domainDataSourceService.CreateDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) QueryDomainDataSource(ctx context.Context, request *datamesh.QueryDomainDataSourceRequest) (*datamesh.QueryDomainDataSourceResponse, error) {
	return h.domainDataSourceService.QueryDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) UpdateDomainData(ctx context.Context, request *datamesh.UpdateDomainDataSourceRequest) (*datamesh.UpdateDomainDataSourceResponse, error) {
	return h.domainDataSourceService.UpdateDomainDataSource(ctx, request), nil
}

func (h *domainDataSourceHandler) DeleteDomainData(ctx context.Context, request *datamesh.DeleteDomainDataSourceRequest) (*datamesh.DeleteDomainDataSourceResponse, error) {
	return h.domainDataSourceService.DeleteDomainDataSource(ctx, request), nil
}

type domainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
	datamesh.UnimplementedDomainDataGrantServiceServer
}

func NewDomainDataGrantHandler(domaindataSourceService service.IDomainDataGrantService) datamesh.DomainDataGrantServiceServer {
	return &domainDataGrantHandler{
		domainDataGrantService: domaindataSourceService,
	}
}

func (h *domainDataGrantHandler) CreateDomainDataGrant(ctx context.Context, request *datamesh.CreateDomainDataGrantRequest) (*datamesh.CreateDomainDataGrantResponse, error) {
	return h.domainDataGrantService.CreateDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) QueryDomainDataGrant(ctx context.Context, request *datamesh.QueryDomainDataGrantRequest) (*datamesh.QueryDomainDataGrantResponse, error) {
	return h.domainDataGrantService.QueryDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) UpdateDomainDataGrant(ctx context.Context, request *datamesh.UpdateDomainDataGrantRequest) (*datamesh.UpdateDomainDataGrantResponse, error) {
	return h.domainDataGrantService.UpdateDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) DeleteDomainDataGrant(ctx context.Context, request *datamesh.DeleteDomainDataGrantRequest) (*datamesh.DeleteDomainDataGrantResponse, error) {
	return h.domainDataGrantService.DeleteDomainDataGrant(ctx, request), nil
}
