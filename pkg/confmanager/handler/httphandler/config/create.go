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

package config

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

type createConfigHandler struct {
	configService service.IConfigService
}

func NewCreateConfigHandler(configService service.IConfigService) api.ProtoHandler {
	return &createConfigHandler{
		configService: configService,
	}
}

func (h createConfigHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h createConfigHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	createRequest, _ := request.(*confmanager.CreateConfigRequest)
	return h.configService.CreateConfig(context.Context, createRequest)
}

func (h createConfigHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(confmanager.CreateConfigRequest{}), reflect.TypeOf(confmanager.CreateConfigResponse{})
}
