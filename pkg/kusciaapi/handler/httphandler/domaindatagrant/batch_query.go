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

package domaindatagrant

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type batchQueryDomainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
}

func NewBatchQueryDomainDataGrantHandler(domainDataGrantService service.IDomainDataGrantService) api.ProtoHandler {
	return &batchQueryDomainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h batchQueryDomainDataGrantHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h batchQueryDomainDataGrantHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	batchRequest, _ := request.(*kusciaapi.BatchQueryDomainDataGrantRequest)
	return h.domainDataGrantService.BatchQueryDomainDataGrant(context.Context, batchRequest)
}

func (h batchQueryDomainDataGrantHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.BatchQueryDomainDataGrantRequest{}), reflect.TypeOf(kusciaapi.BatchQueryDomainDataGrantResponse{})
}
