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

package domaindatasource

import (
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type createDomainDataSourceHandler struct {
	domainDataSourceService service.IDomainDataSourceService
}

func NewCreateDomainDataSourceHandler(domainDataSourceService service.IDomainDataSourceService) api.ProtoHandler {
	return &createDomainDataSourceHandler{
		domainDataSourceService: domainDataSourceService,
	}
}

func (h *createDomainDataSourceHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	createReq, _ := request.(*datamesh.CreateDomainDataSourceRequest)
	if createReq.Info == nil {
		errs.AppendErr(errors.New("datasource info should not be empty"))
	}
	if createReq.Type == "" {
		errs.AppendErr(errors.New("type should not be empty"))
	}
}

func (h *createDomainDataSourceHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	createRequest, _ := request.(*datamesh.CreateDomainDataSourceRequest)
	return h.domainDataSourceService.CreateDomainDataSource(context.Context, createRequest)
}

func (h *createDomainDataSourceHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.CreateDomainDataSourceRequest{}), reflect.TypeOf(datamesh.CreateDomainDataSourceResponse{})
}
