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

type domainHandler struct {
	domainService service.IDomainService
	kusciaapi.UnimplementedDomainServiceServer
}

func NewDomainHandler(domainService service.IDomainService) kusciaapi.DomainServiceServer {
	return &domainHandler{
		domainService: domainService,
	}
}

func (h domainHandler) CreateDomain(ctx context.Context, request *kusciaapi.CreateDomainRequest) (*kusciaapi.CreateDomainResponse, error) {
	return h.domainService.CreateDomain(ctx, request), nil
}

func (h domainHandler) QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) (*kusciaapi.QueryDomainResponse, error) {
	return h.domainService.QueryDomain(ctx, request), nil
}

func (h domainHandler) UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) (*kusciaapi.UpdateDomainResponse, error) {
	return h.domainService.UpdateDomain(ctx, request), nil
}

func (h domainHandler) DeleteDomain(ctx context.Context, request *kusciaapi.DeleteDomainRequest) (*kusciaapi.DeleteDomainResponse, error) {
	return h.domainService.DeleteDomain(ctx, request), nil
}

func (h domainHandler) BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) (*kusciaapi.BatchQueryDomainResponse, error) {
	return h.domainService.BatchQueryDomain(ctx, request), nil
}
