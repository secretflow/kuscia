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

	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type domainRouteServiceLite struct {
	kusciaAPIClient proxy.KusciaAPIClient
}

func (s domainRouteServiceLite) CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) *kusciaapi.CreateDomainRouteResponse {
	// do validate
	if err := validateCreateDomainRouteRequest(request); err != nil {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.CreateDomainRoute(ctx, request)
	if err != nil {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s domainRouteServiceLite) DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) *kusciaapi.DeleteDomainRouteResponse {
	// do validate
	if err := validateDomainRouteRequest(request); err != nil {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.DeleteDomainRoute(ctx, request)
	if err != nil {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s domainRouteServiceLite) QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) *kusciaapi.QueryDomainRouteResponse {
	// do validate
	if err := validateDomainRouteRequest(request); err != nil {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.QueryDomainRoute(ctx, request)
	if err != nil {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s domainRouteServiceLite) BatchQueryDomainRouteStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) *kusciaapi.BatchQueryDomainRouteStatusResponse {
	// do validate
	routeKeys := request.RouteKeys
	if len(routeKeys) == 0 {
		return &kusciaapi.BatchQueryDomainRouteStatusResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "DomainRoute keys can not be empty"),
		}
	}
	for i, key := range routeKeys {
		if err := validateDomainRouteRequest(key); err != nil {
			nlog.Errorf("Validate BatchQueryDomainRouteStatusRequest the index: %d of route key, failed: %s.", i, err.Error())
			return &kusciaapi.BatchQueryDomainRouteStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
	}
	// request the master api
	resp, err := s.kusciaAPIClient.BatchQueryDomainRoute(ctx, request)
	if err != nil {
		return &kusciaapi.BatchQueryDomainRouteStatusResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}
