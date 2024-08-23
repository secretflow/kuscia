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

type modelHandler struct {
	modelService service.IModelService
	kusciaapi.UnimplementedModelServiceServer
}

func NewModelHandler(modelService service.IModelService) kusciaapi.ModelServiceServer {
	return &modelHandler{
		modelService: modelService,
	}
}

func (h modelHandler) UploadModel(ctx context.Context, request *kusciaapi.UploadModelRequest) (*kusciaapi.UploadModelResponse, error) {
	return h.modelService.UploadModel(ctx, request), nil
}