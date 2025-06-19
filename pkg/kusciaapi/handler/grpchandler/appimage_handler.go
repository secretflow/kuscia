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

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type appImageHandler struct {
	appImageService service.IAppImageService
	kusciaapi.UnimplementedAppImageServiceServer
}

func NewAppImageHandler(appImageService service.IAppImageService) kusciaapi.AppImageServiceServer {
	return &appImageHandler{
		appImageService: appImageService,
	}
}

func (h appImageHandler) CreateAppImage(ctx context.Context, request *kusciaapi.CreateAppImageRequest) (*kusciaapi.CreateAppImageResponse, error) {
	return h.appImageService.CreateAppImage(ctx, request), nil
}

func (h appImageHandler) QueryAppImage(ctx context.Context, request *kusciaapi.QueryAppImageRequest) (*kusciaapi.QueryAppImageResponse, error) {
	return h.appImageService.QueryAppImage(ctx, request), nil
}

func (h appImageHandler) ListAppImage(ctx context.Context, request *kusciaapi.ListAppImageRequest) (*kusciaapi.ListAppImageResponse, error) {
	return h.appImageService.ListAppImage(ctx, request), nil
}

func (h appImageHandler) UpdateAppImage(ctx context.Context, request *kusciaapi.UpdateAppImageRequest) (*kusciaapi.UpdateAppImageResponse, error) {
	return h.appImageService.UpdateAppImage(ctx, request), nil
}

func (h appImageHandler) DeleteAppImage(ctx context.Context, request *kusciaapi.DeleteAppImageRequest) (*kusciaapi.DeleteAppImageResponse, error) {
	return h.appImageService.DeleteAppImage(ctx, request), nil
}

func (h appImageHandler) BatchQueryAppImage(ctx context.Context, request *kusciaapi.BatchQueryAppImageRequest) (*kusciaapi.BatchQueryAppImageResponse, error) {
	return h.appImageService.BatchQueryAppImage(ctx, request), nil
}
