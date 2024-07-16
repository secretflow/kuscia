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

type servingHandler struct {
	servingService service.IServingService
	kusciaapi.UnimplementedServingServiceServer
}

func NewServingHandler(servingService service.IServingService) kusciaapi.ServingServiceServer {
	return &servingHandler{
		servingService: servingService,
	}
}

func (h servingHandler) CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) (*kusciaapi.CreateServingResponse, error) {
	return h.servingService.CreateServing(ctx, request), nil
}

func (h servingHandler) QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) (*kusciaapi.QueryServingResponse, error) {
	return h.servingService.QueryServing(ctx, request), nil
}

func (h servingHandler) UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) (*kusciaapi.UpdateServingResponse, error) {
	return h.servingService.UpdateServing(ctx, request), nil
}

func (h servingHandler) DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) (*kusciaapi.DeleteServingResponse, error) {
	return h.servingService.DeleteServing(ctx, request), nil
}

func (h servingHandler) BatchQueryServingStatus(ctx context.Context, request *kusciaapi.BatchQueryServingStatusRequest) (*kusciaapi.BatchQueryServingStatusResponse, error) {
	return h.servingService.BatchQueryServingStatus(ctx, request), nil
}
