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

package configuration

import (
	"fmt"
	"reflect"

	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

type createConfigurationHandler struct {
	configurationService service.IConfigurationService
}

func NewCreateConfigurationHandler(configurationService service.IConfigurationService) api.ProtoHandler {
	return &createConfigurationHandler{
		configurationService: configurationService,
	}
}

func (h createConfigurationHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	createRequest, _ := request.(*confmanager.CreateConfigurationRequest)
	if err := validateTLSOu(context); err != nil {
		errs.AppendErr(err)
	}
	if createRequest.Id == "" {
		errs.AppendErr(fmt.Errorf("confId must not empty"))
	}
}

func (h createConfigurationHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	createRequest, _ := request.(*confmanager.CreateConfigurationRequest)
	tlsCert := interceptor.TLSCertFromGinContext(context.Context)
	return h.configurationService.CreateConfiguration(context.Context, createRequest, tlsCert.OrganizationalUnit[0])
}

func (h createConfigurationHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(confmanager.CreateConfigurationRequest{}), reflect.TypeOf(confmanager.CreateConfigurationResponse{})
}
