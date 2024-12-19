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

package service

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type appImageServiceLite struct {
	kusciaAPIClient proxy.KusciaAPIClient
}

func (s appImageServiceLite) CreateAppImage(ctx context.Context, request *kusciaapi.CreateAppImageRequest) *kusciaapi.CreateAppImageResponse {
	// kuscia lite api not support this interface
	return &kusciaapi.CreateAppImageResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s appImageServiceLite) QueryAppImage(ctx context.Context, request *kusciaapi.QueryAppImageRequest) *kusciaapi.QueryAppImageResponse {
	return &kusciaapi.QueryAppImageResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s appImageServiceLite) UpdateAppImage(ctx context.Context, request *kusciaapi.UpdateAppImageRequest) *kusciaapi.UpdateAppImageResponse {
	return &kusciaapi.UpdateAppImageResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s appImageServiceLite) DeleteAppImage(ctx context.Context, request *kusciaapi.DeleteAppImageRequest) *kusciaapi.DeleteAppImageResponse {
	// kuscia lite api not support this interface
	return &kusciaapi.DeleteAppImageResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s appImageServiceLite) BatchQueryAppImage(ctx context.Context, request *kusciaapi.BatchQueryAppImageRequest) *kusciaapi.BatchQueryAppImageResponse {
	// kuscia lite api not support this interface
	return &kusciaapi.BatchQueryAppImageResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}
