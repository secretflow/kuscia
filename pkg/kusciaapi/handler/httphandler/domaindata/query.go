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

//nolint:dulp
package domaindata

import (
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
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
}

func (h *queryDomainDataHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req, _ := request.(*kusciaapi.QueryDomainDataRequest)
	return h.domainDataService.QueryDomainData(context.Context, req)
}

func (h *queryDomainDataHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.QueryDomainDataRequest{}), reflect.TypeOf(kusciaapi.QueryDomainDataResponse{})
}

// Batch Query
type batchQueryDomainDataHandler struct {
	domainDataService service.IDomainDataService
}

func NewBatchQueryDomainDataHandler(domainDataService service.IDomainDataService) api.ProtoHandler {
	return &batchQueryDomainDataHandler{
		domainDataService: domainDataService,
	}
}

func (h *batchQueryDomainDataHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, _ := request.(*kusciaapi.BatchQueryDomainDataRequest)
	if req.Data == nil || len(req.Data) == 0 {
		errs.AppendErr(errors.New("request data should not be nil"))
		return
	}
	for _, v := range req.Data {
		if v.DomainId == "" {
			errs.AppendErr(errors.New("request domainID should not be empty"))
			break
		}
		if v.DomaindataId == "" {
			errs.AppendErr(errors.New("request domainDataID should not be empty"))
			break
		}
	}
}

func (h *batchQueryDomainDataHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req, _ := request.(*kusciaapi.BatchQueryDomainDataRequest)
	return h.domainDataService.BatchQueryDomainData(context.Context, req)
}

func (h *batchQueryDomainDataHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.BatchQueryDomainDataRequest{}), reflect.TypeOf(kusciaapi.BatchQueryDomainDataResponse{})
}

// List
type listDomainDataHandler struct {
	domainDataService service.IDomainDataService
}

func NewListDomainDataHandler(domainDataService service.IDomainDataService) api.ProtoHandler {
	return &listDomainDataHandler{
		domainDataService: domainDataService,
	}
}

func (h *listDomainDataHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, _ := request.(*kusciaapi.ListDomainDataRequest)
	if req.Data == nil {
		errs.AppendErr(errors.New("request data should not be nil"))
		return
	}
	if req.Data.DomainId == "" {
		errs.AppendErr(errors.New("request domainID should not be empty"))
	}
}

func (h *listDomainDataHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req, _ := request.(*kusciaapi.ListDomainDataRequest)
	return h.domainDataService.ListDomainData(context.Context, req)
}

func (h *listDomainDataHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.ListDomainDataRequest{}), reflect.TypeOf(kusciaapi.ListDomainDataResponse{})
}
