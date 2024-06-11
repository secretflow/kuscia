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

//nolint:dulp
package domaindatagrant

import (
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type queryDomainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
}

func NewQueryDomainDataGrantHandler(domainDataGrantService service.IDomainDataGrantService) api.ProtoHandler {
	return &queryDomainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h *queryDomainDataGrantHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	queryReq, _ := request.(*datamesh.QueryDomainDataGrantRequest)
	if queryReq.DomaindatagrantId == "" {
		errs.AppendErr(errors.New("domaindatagrantid should not be empty"))
	}
}

func (h *queryDomainDataGrantHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	queryRequest, _ := request.(*datamesh.QueryDomainDataGrantRequest)
	return h.domainDataGrantService.QueryDomainDataGrant(context.Context, queryRequest)
}

func (h *queryDomainDataGrantHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.QueryDomainDataGrantRequest{}), reflect.TypeOf(datamesh.QueryDomainDataGrantResponse{})
}
