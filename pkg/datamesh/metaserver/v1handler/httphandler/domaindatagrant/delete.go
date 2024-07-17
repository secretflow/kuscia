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

	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type deleteDomainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
}

func NewDeleteDomainDataGrantHandler(domainDataGrantService service.IDomainDataGrantService) api.ProtoHandler {
	return &deleteDomainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h *deleteDomainDataGrantHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	deleteReq, _ := request.(*datamesh.DeleteDomainDataGrantRequest)
	if deleteReq.DomaindatagrantId == "" {
		errs.AppendErr(errors.New("domaindatagrant id should not be empty"))
	}
}

func (h *deleteDomainDataGrantHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	deleteRequest, _ := request.(*datamesh.DeleteDomainDataGrantRequest)
	return h.domainDataGrantService.DeleteDomainDataGrant(context.Context, deleteRequest)
}

func (h *deleteDomainDataGrantHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.DeleteDomainDataGrantRequest{}), reflect.TypeOf(datamesh.DeleteDomainDataGrantResponse{})
}
