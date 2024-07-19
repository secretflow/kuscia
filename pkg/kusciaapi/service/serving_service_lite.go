// Copyright 2024 Ant Group Co., Ltd.
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

	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	utils2 "github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type servingServiceLite struct {
	Initiator       string
	kusciaAPIClient proxy.KusciaAPIClient
}

func (s *servingServiceLite) CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) *kusciaapi.CreateServingResponse {
	// do validate
	if err := validateCreateServingRequest(s.Initiator, request); err != nil {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	// request the master api
	resp, err := s.kusciaAPIClient.CreateServing(ctx, request)
	if err != nil {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s *servingServiceLite) QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) *kusciaapi.QueryServingResponse {
	// do validate
	servingID := request.ServingId
	if servingID == "" {
		return &kusciaapi.QueryServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "serving id can not be empty"),
		}
	}

	// request the master api
	resp, err := s.kusciaAPIClient.QueryServing(ctx, request)
	if err != nil {
		return &kusciaapi.QueryServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s *servingServiceLite) BatchQueryServingStatus(ctx context.Context,
	request *kusciaapi.BatchQueryServingStatusRequest) *kusciaapi.BatchQueryServingStatusResponse {
	// do validate
	if err := validateBatchQueryServingStatusRequest(request); err != nil {
		return &kusciaapi.BatchQueryServingStatusResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	// request the master api
	resp, err := s.kusciaAPIClient.BatchQueryServing(ctx, request)
	if err != nil {
		return &kusciaapi.BatchQueryServingStatusResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s *servingServiceLite) UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) *kusciaapi.UpdateServingResponse {
	if request.ServingId == "" {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "serving id can not be empty"),
		}
	}

	if request.ServingInputConfig == "" && len(request.Parties) == 0 {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildSuccessResponseStatus(),
		}
	}

	// request the master api
	resp, err := s.kusciaAPIClient.UpdateServing(ctx, request)
	if err != nil {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s *servingServiceLite) DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) *kusciaapi.DeleteServingResponse {
	// do validate
	servingID := request.ServingId
	if servingID == "" {
		return &kusciaapi.DeleteServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "serving id can not be empty"),
		}
	}

	// request the master api
	resp, err := s.kusciaAPIClient.DeleteServing(ctx, request)
	if err != nil {
		return &kusciaapi.DeleteServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}
