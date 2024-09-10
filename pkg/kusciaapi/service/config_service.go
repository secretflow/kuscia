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
	"fmt"

	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	utils2 "github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IConfigService interface {
	CreateConfig(ctx context.Context, request *kusciaapi.CreateConfigRequest) *kusciaapi.CreateConfigResponse
	QueryConfig(ctx context.Context, request *kusciaapi.QueryConfigRequest) *kusciaapi.QueryConfigResponse
	BatchQueryConfig(ctx context.Context, request *kusciaapi.BatchQueryConfigRequest) *kusciaapi.BatchQueryConfigResponse
	UpdateConfig(ctx context.Context, request *kusciaapi.UpdateConfigRequest) *kusciaapi.UpdateConfigResponse
	DeleteConfig(ctx context.Context, request *kusciaapi.DeleteConfigRequest) *kusciaapi.DeleteConfigResponse
}

type configService struct {
	conf            *config.KusciaAPIConfig
	cmConfigService cmservice.IConfigService
}

func NewConfigService(conf *config.KusciaAPIConfig, cmConfigService cmservice.IConfigService) IConfigService {
	return &configService{
		conf:            conf,
		cmConfigService: cmConfigService,
	}
}

func (s *configService) CreateConfig(ctx context.Context, request *kusciaapi.CreateConfigRequest) *kusciaapi.CreateConfigResponse {
	if len(request.Data) == 0 {
		return &kusciaapi.CreateConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "request data can't be empty"),
		}
	}

	cmReq, err := buildCMCreateConfigRequest(request)
	if err != nil {
		return &kusciaapi.CreateConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	cmResp := s.cmConfigService.CreateConfig(ctx, cmReq)
	if cmResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return &kusciaapi.CreateConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrCreateConfig, cmResp.Status.Message),
		}
	}

	return &kusciaapi.CreateConfigResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}

func buildCMCreateConfigRequest(req *kusciaapi.CreateConfigRequest) (*confmanager.CreateConfigRequest, error) {
	var cmData []*confmanager.ConfigData
	for i, d := range req.Data {
		if d.Key == "" {
			return nil, fmt.Errorf("request data[%v].key can't be empty", i)
		}
		cmData = append(cmData, &confmanager.ConfigData{
			Key:   d.Key,
			Value: d.Value,
		})
	}
	return &confmanager.CreateConfigRequest{Data: cmData}, nil
}

func (s *configService) QueryConfig(ctx context.Context, request *kusciaapi.QueryConfigRequest) *kusciaapi.QueryConfigResponse {
	if request.Key == "" {
		return &kusciaapi.QueryConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "request key can't be empty"),
		}
	}

	cmReq := &confmanager.QueryConfigRequest{Key: request.Key}
	cmResp := s.cmConfigService.QueryConfig(ctx, cmReq)
	if cmResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return &kusciaapi.QueryConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryConfig, cmResp.Status.Message),
		}
	}

	return &kusciaapi.QueryConfigResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Key:    cmResp.Key,
		Value:  cmResp.Value,
	}
}

func (s *configService) BatchQueryConfig(ctx context.Context, request *kusciaapi.BatchQueryConfigRequest) *kusciaapi.BatchQueryConfigResponse {
	if len(request.Keys) == 0 {
		return &kusciaapi.BatchQueryConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "request keys can't be empty"),
		}
	}

	for i, key := range request.Keys {
		if key == "" {
			return &kusciaapi.BatchQueryConfigResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, fmt.Sprintf("request keys[%v] can't be empty", i)),
			}
		}
	}

	cmReq := &confmanager.BatchQueryConfigRequest{Keys: request.Keys}
	cmResp := s.cmConfigService.BatchQueryConfig(ctx, cmReq)
	if cmResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return &kusciaapi.BatchQueryConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrBatchQueryConfig, cmResp.Status.Message),
		}
	}

	return &kusciaapi.BatchQueryConfigResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data:   buildKusciaAPIBatchQueryConfigRespData(cmResp.Data),
	}
}

func buildKusciaAPIBatchQueryConfigRespData(cmData []*confmanager.ConfigData) []*kusciaapi.ConfigData {
	var data []*kusciaapi.ConfigData
	for _, d := range cmData {
		data = append(data, &kusciaapi.ConfigData{
			Key:   d.Key,
			Value: d.Value,
		})
	}
	return data
}

func (s *configService) UpdateConfig(ctx context.Context, request *kusciaapi.UpdateConfigRequest) *kusciaapi.UpdateConfigResponse {
	if len(request.Data) == 0 {
		return &kusciaapi.UpdateConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "request data can't be empty"),
		}
	}

	for i, data := range request.Data {
		if data.Key == "" {
			return &kusciaapi.UpdateConfigResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, fmt.Sprintf("request data[%v].key can't be empty", i)),
			}
		}
	}

	cmReq := buildCMUpdateConfigRequest(request)
	cmResp := s.cmConfigService.UpdateConfig(ctx, cmReq)
	if cmResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return &kusciaapi.UpdateConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrUpdateConfig, cmResp.Status.Message),
		}
	}

	return &kusciaapi.UpdateConfigResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}

func buildCMUpdateConfigRequest(req *kusciaapi.UpdateConfigRequest) *confmanager.UpdateConfigRequest {
	var data []*confmanager.ConfigData
	for _, d := range req.Data {
		data = append(data, &confmanager.ConfigData{Key: d.Key, Value: d.Value})
	}
	return &confmanager.UpdateConfigRequest{Data: data}
}

func (s *configService) DeleteConfig(ctx context.Context, request *kusciaapi.DeleteConfigRequest) *kusciaapi.DeleteConfigResponse {
	if len(request.Keys) == 0 {
		return &kusciaapi.DeleteConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, "request keys can't be empty"),
		}
	}

	for i, key := range request.Keys {
		if key == "" {
			return &kusciaapi.DeleteConfigResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, fmt.Sprintf("request keys[%v] can't be empty", i)),
			}
		}
	}

	cmResp := s.cmConfigService.DeleteConfig(ctx, &confmanager.DeleteConfigRequest{
		Keys: request.Keys,
	})

	if cmResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return &kusciaapi.DeleteConfigResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrDeleteConfig, cmResp.Status.Message),
		}
	}

	return &kusciaapi.DeleteConfigResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}
