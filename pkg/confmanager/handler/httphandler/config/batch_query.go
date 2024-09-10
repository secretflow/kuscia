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

type batchQueryConfigHandler struct {
	configService service.IConfigService
}

func NewBatchQueryConfigHandler(configService service.IConfigService) api.ProtoHandler {
	return &batchQueryConfigHandler{
		configService: configService,
	}
}

func (h batchQueryConfigHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h batchQueryConfigHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	batchQueryRequest, _ := request.(*confmanager.BatchQueryConfigRequest)
	return h.configService.BatchQueryConfig(context.Context, batchQueryRequest)
}

func (h batchQueryConfigHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(confmanager.BatchQueryConfigRequest{}), reflect.TypeOf(confmanager.BatchQueryConfigResponse{})
}
