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

package model

import (
	"io"
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type uploadModelHandler struct {
	modelService service.IModelService
}

func NewUploadModelHandler(modelService service.IModelService) api.ProtoHandler {
	return &uploadModelHandler{
		modelService: modelService,
	}
}

func (h uploadModelHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h uploadModelHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	context.Request.ParseMultipartForm(10 << 20)
	Filename := context.PostForm("Filename")
	tmpFile, _, _ := context.Request.FormFile("Content")
	defer tmpFile.Close()
	bytes, _ := io.ReadAll(tmpFile)

	uploadRequest, _ := request.(*kusciaapi.UploadModelRequest)
	uploadRequest.Filename = Filename
	uploadRequest.Content = bytes
	return h.modelService.UploadModel(context.Context, uploadRequest)
}

func (h uploadModelHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.UploadModelRequest{}), reflect.TypeOf(kusciaapi.UploadModelResponse{})
}
