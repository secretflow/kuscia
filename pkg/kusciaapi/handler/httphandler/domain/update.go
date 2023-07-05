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

package domain

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type updateDomainHandler struct {
	domainService service.IDomainService
}

func NewUpdateDomainHandler(domainService service.IDomainService) api.ProtoHandler {
	return &updateDomainHandler{
		domainService: domainService,
	}
}

func (h updateDomainHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h updateDomainHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	updateRequest, _ := request.(*kusciaapi.UpdateDomainRequest)
	return h.domainService.UpdateDomain(context.Context, updateRequest)
}

func (h updateDomainHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.UpdateDomainRequest{}), reflect.TypeOf(kusciaapi.UpdateDomainResponse{})
}
