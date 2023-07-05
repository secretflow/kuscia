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

type updateDomainDataSourceHandler struct {
	domainDataSourceService service.IDomainDataSourceService
}

func NewUpdateDomainSourceHandler(domainDataSourceService service.IDomainDataSourceService) api.ProtoHandler {
	return &updateDomainDataSourceHandler{
		domainDataSourceService: domainDataSourceService,
	}
}

func (h *updateDomainDataSourceHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	updateReq, _ := request.(*datamesh.UpdateDomainDataSourceRequest)
	if updateReq.DatasourceId == "" {
		errs.AppendErr(errors.New("domaindata id should not be empty"))
	}
}

func (h *updateDomainDataSourceHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	updateRequest, _ := request.(*datamesh.UpdateDomainDataSourceRequest)
	return h.domainDataSourceService.UpdateDomainDataSource(context.Context, updateRequest)
}

func (h *updateDomainDataSourceHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.UpdateDomainDataSourceRequest{}), reflect.TypeOf(datamesh.UpdateDomainDataSourceResponse{})
}

// Delete handler
type deleteDomainDataSourceHandler struct {
	domainDataSourceService service.IDomainDataSourceService
}

func NewDeleteDomainDataSourceHandler(domainDataSourceService service.IDomainDataSourceService) api.ProtoHandler {
	return &deleteDomainDataSourceHandler{
		domainDataSourceService: domainDataSourceService,
	}
}

func (h *deleteDomainDataSourceHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	deleteReq, _ := request.(*datamesh.DeleteDomainDataSourceRequest)
	if deleteReq.DatasourceId == "" {
		errs.AppendErr(errors.New("datasource id should not be empty"))
	}
}

func (h *deleteDomainDataSourceHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	deleteRequest, _ := request.(*datamesh.DeleteDomainDataSourceRequest)
	return h.domainDataSourceService.DeleteDomainDataSource(context.Context, deleteRequest)
}

func (h *deleteDomainDataSourceHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.DeleteDomainDataSourceRequest{}), reflect.TypeOf(datamesh.DeleteDomainDataSourceResponse{})
}
