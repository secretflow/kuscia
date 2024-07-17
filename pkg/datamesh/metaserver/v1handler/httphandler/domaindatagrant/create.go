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

type createDomainDataGrantHandler struct {
	domainDataGrantService service.IDomainDataGrantService
}

func NewCreateDomainDataGrantHandler(domainDataGrantService service.IDomainDataGrantService) api.ProtoHandler {
	return &createDomainDataGrantHandler{
		domainDataGrantService: domainDataGrantService,
	}
}

func (h *createDomainDataGrantHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	createReq, _ := request.(*datamesh.CreateDomainDataGrantRequest)
	if createReq.DomaindataId == "" {
		errs.AppendErr(errors.New("domaindataid should not be empty"))
	}
	if createReq.GrantDomain == "" {
		errs.AppendErr(errors.New("grantdomain not be empty"))
	}
}

func (h *createDomainDataGrantHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	createRequest, _ := request.(*datamesh.CreateDomainDataGrantRequest)
	return h.domainDataGrantService.CreateDomainDataGrant(context.Context, createRequest)
}

func (h *createDomainDataGrantHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(datamesh.CreateDomainDataGrantRequest{}), reflect.TypeOf(datamesh.CreateDomainDataGrantResponse{})
}
