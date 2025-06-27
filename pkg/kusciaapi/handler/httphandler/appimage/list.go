// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package appimage

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type listAppImageHandler struct {
	appImageService service.IAppImageService
}

func NewListAppImageHandler(appImageService service.IAppImageService) api.ProtoHandler {
	return &listAppImageHandler{
		appImageService: appImageService,
	}
}

func (c *listAppImageHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {

}

func (c *listAppImageHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	listRequest, _ := request.(*kusciaapi.ListAppImageRequest)
	return c.appImageService.ListAppImage(context.Context, listRequest)
}

func (c *listAppImageHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.ListAppImageRequest{}), reflect.TypeOf(kusciaapi.ListAppImageResponse{})
}
