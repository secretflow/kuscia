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

type domainRouteHandler struct {
	domainRouteService service.IDomainRouteService
	kusciaapi.UnimplementedDomainRouteServiceServer
}

func NewDomainRouteHandler(domainRouteService service.IDomainRouteService) kusciaapi.DomainRouteServiceServer {
	return &domainRouteHandler{
		domainRouteService: domainRouteService,
	}
}

func (h domainRouteHandler) CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) (*kusciaapi.CreateDomainRouteResponse, error) {
	return h.domainRouteService.CreateDomainRoute(ctx, request), nil
}

func (h domainRouteHandler) DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) (*kusciaapi.DeleteDomainRouteResponse, error) {
	return h.domainRouteService.DeleteDomainRoute(ctx, request), nil
}

func (h domainRouteHandler) QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) (*kusciaapi.QueryDomainRouteResponse, error) {
	return h.domainRouteService.QueryDomainRoute(ctx, request), nil
}

func (h domainRouteHandler) BatchQueryDomainRouteStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) (*kusciaapi.BatchQueryDomainRouteStatusResponse, error) {
	return h.domainRouteService.BatchQueryDomainRouteStatus(ctx, request), nil
}

func (h domainRouteHandler) mustEmbedUnimplementedRouteServiceServer() {
}
