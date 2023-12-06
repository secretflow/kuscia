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

type queryConfigurationHandler struct {
	configurationService service.IConfigurationService
}

func NewQueryConfigurationHandler(configurationService service.IConfigurationService) api.ProtoHandler {
	return &queryConfigurationHandler{
		configurationService: configurationService,
	}
}

func (h queryConfigurationHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	queryRequest, _ := request.(*confmanager.QueryConfigurationRequest)
	if err := validateTLSOu(context); err != nil {
		errs.AppendErr(err)
	}
	if len(queryRequest.Ids) == 0 {
		errs.AppendErr(fmt.Errorf("confIds must not empty"))
	}
}

func (h queryConfigurationHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	queryRequest, _ := request.(*confmanager.QueryConfigurationRequest)
	tlsCert := interceptor.TLSCertFromGinContext(context.Context)
	response := h.configurationService.QueryConfiguration(context.Context, queryRequest, tlsCert.OrganizationalUnit[0])
	return &response
}

func (h queryConfigurationHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(confmanager.QueryConfigurationRequest{}), reflect.TypeOf(confmanager.QueryConfigurationResponse{})
}
