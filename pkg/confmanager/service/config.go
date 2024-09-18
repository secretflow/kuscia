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
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/driver"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

// IConfigService manager configs.
type IConfigService interface {
	CreateConfig(context.Context, *confmanager.CreateConfigRequest) *confmanager.CreateConfigResponse
	QueryConfig(context.Context, *confmanager.QueryConfigRequest) *confmanager.QueryConfigResponse
	UpdateConfig(context.Context, *confmanager.UpdateConfigRequest) *confmanager.UpdateConfigResponse
	DeleteConfig(context.Context, *confmanager.DeleteConfigRequest) *confmanager.DeleteConfigResponse
	BatchQueryConfig(context.Context, *confmanager.BatchQueryConfigRequest) *confmanager.BatchQueryConfigResponse
}

type configService struct {
	driver driver.Driver
}

type ConfigServiceConfig struct {
	DomainID     string
	DomainKey    *rsa.PrivateKey
	Driver       string
	DisableCache bool
	KubeClient   kubernetes.Interface
}

func NewConfigService(ctx context.Context, conf *ConfigServiceConfig) (IConfigService, error) {
	cmDriver, err := driver.NewDriver(ctx, &driver.Config{
		DomainID:     conf.DomainID,
		DomainKey:    conf.DomainKey,
		Driver:       conf.Driver,
		DisableCache: conf.DisableCache,
		ConfigName:   config.DomainConfigName,
		KubeClient:   conf.KubeClient,
	})
	if err != nil {
		return nil, err
	}

	// set domain public key into domain-config
	value, _, err := cmDriver.GetConfig(ctx, config.DomainPublicKeyName)
	if err != nil {
		nlog.Errorf("Failed to get domain public key config, %v", err.Error())
	}
	if value == "" {
		pubKey, err := generateBase64PublicKeyStr(conf.DomainKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encode public key to base64 format, %v", err.Error())
		}
		if err = cmDriver.SetConfig(ctx, map[string]string{config.DomainPublicKeyName: pubKey}); err != nil {
			nlog.Errorf("Failed to set domain public key in domain-config, %v", err.Error())
		}
	}

	return &configService{
		driver: cmDriver,
	}, nil
}

func generateBase64PublicKeyStr(priKey *rsa.PrivateKey) (string, error) {
	var err error
	publicKeyPem := tls.EncodePKCS1PublicKey(priKey)
	if len(publicKeyPem) == 0 {
		publicKeyPem, err = tls.EncodePKCS8PublicKey(priKey)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(publicKeyPem), nil
}

func (s *configService) CreateConfig(ctx context.Context, request *confmanager.CreateConfigRequest) *confmanager.CreateConfigResponse {
	if len(request.Data) == 0 {
		return &confmanager.CreateConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, "request data can't be empty"),
		}
	}

	data := make(map[string]string)
	for i, d := range request.Data {
		if d.Key == "" {
			return &confmanager.CreateConfigResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, fmt.Sprintf("request data[%v].key can't be empty", i)),
			}
		}
		_, exist, err := s.driver.GetConfig(ctx, d.Key)
		if err != nil {
			return &confmanager.CreateConfigResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrCreateConfig, fmt.Sprintf("get key[%v] value failed, %v", d.Key, err.Error())),
			}
		}
		if exist {
			return &confmanager.CreateConfigResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, fmt.Sprintf("request key[%v] already exist, duplicate creation is not allowed", d.Key)),
			}
		}
		data[d.Key] = d.Value
	}

	if err := s.driver.SetConfig(ctx, data); err != nil {
		return &confmanager.CreateConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrCreateConfig, err.Error()),
		}
	}

	return &confmanager.CreateConfigResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *configService) QueryConfig(ctx context.Context, request *confmanager.QueryConfigRequest) *confmanager.QueryConfigResponse {
	if request.Key == "" {
		return &confmanager.QueryConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, "request key can't be empty"),
		}
	}

	value, _, err := s.driver.GetConfig(ctx, request.Key)
	if err != nil {
		return &confmanager.QueryConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrQueryConfig, err.Error()),
		}
	}

	return &confmanager.QueryConfigResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Key:    request.Key,
		Value:  value,
	}
}

func (s *configService) UpdateConfig(ctx context.Context, request *confmanager.UpdateConfigRequest) *confmanager.UpdateConfigResponse {
	if len(request.Data) == 0 {
		return &confmanager.UpdateConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, "request data can't be empty"),
		}
	}

	data := make(map[string]string)
	for i, d := range request.Data {
		if d.Key == "" {
			return &confmanager.UpdateConfigResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, fmt.Sprintf("request data[%d].key can't be empty", i)),
			}
		}
		data[d.Key] = d.Value
	}

	if err := s.driver.SetConfig(ctx, data); err != nil {
		return &confmanager.UpdateConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrUpdateConfig, err.Error()),
		}
	}

	return &confmanager.UpdateConfigResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *configService) DeleteConfig(ctx context.Context, request *confmanager.DeleteConfigRequest) *confmanager.DeleteConfigResponse {
	if len(request.Keys) == 0 {
		return &confmanager.DeleteConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, "request keys can't be empty"),
		}
	}

	if err := s.driver.DeleteConfig(ctx, request.Keys); err != nil {
		return &confmanager.DeleteConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrDeleteConfig, err.Error()),
		}
	}

	return &confmanager.DeleteConfigResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *configService) BatchQueryConfig(ctx context.Context, request *confmanager.BatchQueryConfigRequest) *confmanager.BatchQueryConfigResponse {
	if len(request.Keys) == 0 {
		return &confmanager.BatchQueryConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrRequestInvalidate, "request keys can't be empty"),
		}
	}

	result, err := s.driver.ListConfig(ctx, request.Keys)
	if err != nil {
		return &confmanager.BatchQueryConfigResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_ConfManagerErrBatchQueryConfig, err.Error()),
		}
	}

	data := make([]*confmanager.ConfigData, 0)
	for k, v := range result {
		data = append(data, &confmanager.ConfigData{Key: k, Value: v})
	}

	return &confmanager.BatchQueryConfigResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   data,
	}
}
