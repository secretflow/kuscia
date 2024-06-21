// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:dupl
package domaindatagrant

import (
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type updateDomainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
}

func NewUpdateDomainSourceHandler(domainDataGrantService service.IDomainDataGrantService) api.ProtoHandler {
	return &updateDomainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h *updateDomainDataGrantHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	updateReq, _ := request.(*datamesh.UpdateDomainDataGrantRequest)
	if updateReq.DomaindatagrantId == "" {
		errs.AppendErr(errors.New("domaindatagrant id should not be empty"))
	}
	if updateReq.DomaindataId == "" {
		errs.AppendErr(errors.New("domaindata id should not be empty"))
	}
}

func (h *updateDomainDataGrantHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	updateRequest, _ := request.(*datamesh.UpdateDomainDataGrantRequest)
	return h.domainDataGrantService.UpdateDomainDataGrant(context.Context, updateRequest)
}

func (h *updateDomainDataGrantHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.UpdateDomainDataGrantRequest{}), reflect.TypeOf(datamesh.UpdateDomainDataGrantResponse{})
}
