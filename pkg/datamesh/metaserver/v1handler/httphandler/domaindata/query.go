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

//nolint:dupl
package domaindata

import (
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type queryDomainDataHandler struct {
	domainDataService service.IDomainDataService
}

func NewQueryDomainDataHandler(domainDataService service.IDomainDataService) api.ProtoHandler {
	return &queryDomainDataHandler{
		domainDataService: domainDataService,
	}
}

func (h *queryDomainDataHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	queryReq, _ := request.(*datamesh.QueryDomainDataRequest)
	if queryReq.DomaindataId == "" {
		errs.AppendErr(errors.New("domaindata id should not be empty"))
	}
}

func (h *queryDomainDataHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	queryRequest, _ := request.(*datamesh.QueryDomainDataRequest)
	return h.domainDataService.QueryDomainData(context.Context, queryRequest)
}

func (h *queryDomainDataHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.QueryDomainDataRequest{}), reflect.TypeOf(datamesh.QueryDomainDataResponse{})
}
