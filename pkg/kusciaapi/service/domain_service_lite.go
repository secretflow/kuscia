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

package service

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type domainServiceLite struct {
	kusciaAPIClient proxy.KusciaAPIClient
}

func (s domainServiceLite) CreateDomain(ctx context.Context, request *kusciaapi.CreateDomainRequest) *kusciaapi.CreateDomainResponse {
	// kuscia lite api not support this interface
	return &kusciaapi.CreateDomainResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s domainServiceLite) QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) *kusciaapi.QueryDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.QueryDomain(ctx, request)
	if err != nil {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s domainServiceLite) UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) *kusciaapi.UpdateDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	role := request.Role
	if role != "" && role != string(v1alpha1.Partner) {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("role is invalid, must be empty or %s", v1alpha1.Partner)),
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.UpdateDomain(ctx, request)
	if err != nil {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}

	return resp
}

func (s domainServiceLite) DeleteDomain(ctx context.Context, request *kusciaapi.DeleteDomainRequest) *kusciaapi.DeleteDomainResponse {
	// kuscia lite api not support this interface
	return &kusciaapi.DeleteDomainResponse{
		Status: utils.BuildErrorResponseStatus(errorcode.ErrLiteAPINotSupport, "kuscia lite api not support this interface now"),
	}
}

func (s domainServiceLite) BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) *kusciaapi.BatchQueryDomainResponse {
	// do validate
	domainIDs := request.DomainIds
	if len(domainIDs) == 0 {
		return &kusciaapi.BatchQueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain ids can not be empty"),
		}
	}
	for i, domainID := range domainIDs {
		if domainID == "" {
			return &kusciaapi.BatchQueryDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("domain id can not be empty on index %d", i)),
			}
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.BatchQueryDomain(ctx, request)
	if err != nil {
		return &kusciaapi.BatchQueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}
