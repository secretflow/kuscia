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

//nolint:dupl
package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type domainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
	kusciaapi.UnimplementedDomainDataGrantServiceServer
}

func NewDomainDataGrantHandler(domainDataGrantService service.IDomainDataGrantService) kusciaapi.DomainDataGrantServiceServer {
	return &domainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h *domainDataGrantHandler) CreateDomainDataGrant(ctx context.Context, request *kusciaapi.CreateDomainDataGrantRequest) (*kusciaapi.CreateDomainDataGrantResponse, error) {
	return h.domainDataGrantService.CreateDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) UpdateDomainDataGrant(ctx context.Context, request *kusciaapi.UpdateDomainDataGrantRequest) (*kusciaapi.UpdateDomainDataGrantResponse, error) {
	return h.domainDataGrantService.UpdateDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) DeleteDomainDataGrant(ctx context.Context, request *kusciaapi.DeleteDomainDataGrantRequest) (*kusciaapi.DeleteDomainDataGrantResponse, error) {
	return h.domainDataGrantService.DeleteDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) QueryDomainDataGrant(ctx context.Context, request *kusciaapi.QueryDomainDataGrantRequest) (*kusciaapi.QueryDomainDataGrantResponse, error) {
	return h.domainDataGrantService.QueryDomainDataGrant(ctx, request), nil
}

func (h *domainDataGrantHandler) BatchQueryDomainDataGrant(ctx context.Context, request *kusciaapi.BatchQueryDomainDataGrantRequest) (*kusciaapi.BatchQueryDomainDataGrantResponse, error) {
	return h.domainDataGrantService.BatchQueryDomainDataGrant(ctx, request), nil
}
