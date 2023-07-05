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

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type domainDataHandler struct {
	domainDataService service.IDomainDataService
	kusciaapi.UnimplementedDomainDataServiceServer
}

func NewDomainDataHandler(domainDataService service.IDomainDataService) kusciaapi.DomainDataServiceServer {
	return &domainDataHandler{
		domainDataService: domainDataService,
	}
}

func (h *domainDataHandler) CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) (*kusciaapi.CreateDomainDataResponse, error) {
	return h.domainDataService.CreateDomainData(ctx, request), nil
}

func (h *domainDataHandler) UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) (*kusciaapi.UpdateDomainDataResponse, error) {
	return h.domainDataService.UpdateDomainData(ctx, request), nil
}

func (h *domainDataHandler) DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) (*kusciaapi.DeleteDomainDataResponse, error) {
	return h.domainDataService.DeleteDomainData(ctx, request), nil
}

func (h *domainDataHandler) QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) (*kusciaapi.QueryDomainDataResponse, error) {
	return h.domainDataService.QueryDomainData(ctx, request), nil
}

func (h *domainDataHandler) BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) (*kusciaapi.BatchQueryDomainDataResponse, error) {
	return h.domainDataService.BatchQueryDomainData(ctx, request), nil
}

func (h *domainDataHandler) ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) (*kusciaapi.ListDomainDataResponse, error) {
	return h.domainDataService.ListDomainData(ctx, request), nil
}
