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

	"github.com/secretflow/kuscia/pkg/confmanager/errorcode"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

const (
	GroupKuscia = "kuscia"
)

// IConfigurationService is service which manager conf from/to secret backend. It can be used by grpc-mtls/https/process-call.
type IConfigurationService interface {
	// CreateConfiguration create a configuration with confID/content/cn.
	CreateConfiguration(context.Context, *confmanager.CreateConfigurationRequest, string) confmanager.CreateConfigurationResponse
	// QueryConfiguration query a configuration with confIDs/cn.
	QueryConfiguration(context.Context, *confmanager.QueryConfigurationRequest, string) confmanager.QueryConfigurationResponse
}

type configurationService struct {
	backend    secretbackend.SecretDriver
	enableAuth bool
}

func NewConfigurationService(backend secretbackend.SecretDriver, enableAuth bool) (IConfigurationService, error) {
	if backend == nil {
		return nil, fmt.Errorf("can not find secret backend for cm")
	}
	return &configurationService{
		backend:    backend,
		enableAuth: enableAuth,
	}, nil
}

func (s *configurationService) CreateConfiguration(ctx context.Context, request *confmanager.CreateConfigurationRequest, ou string) confmanager.CreateConfigurationResponse {
	if ou == "" {
		return confmanager.CreateConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "tls ou must not be empty"),
		}
	}

	if request.Id == "" {
		return confmanager.CreateConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "confID must not be empty"),
		}
	}

	if ou != GroupKuscia {
		if err := s.backend.Set(s.groupPermissionKey(ou, request.Id), ""); err != nil {
			return confmanager.CreateConfigurationResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrCreateConfiguration, err.Error()),
			}
		}
	}

	if err := s.backend.Set(request.Id, request.Content); err != nil {
		return confmanager.CreateConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrCreateConfiguration, err.Error()),
		}
	}

	return confmanager.CreateConfigurationResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

// QueryConfiguration query configuration
func (s *configurationService) QueryConfiguration(ctx context.Context, request *confmanager.QueryConfigurationRequest, ou string) confmanager.QueryConfigurationResponse {
	if ou == "" {
		return confmanager.QueryConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "tls ou must not be empty"),
		}
	}

	if len(request.GetIds()) == 0 {
		return confmanager.QueryConfigurationResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestInvalidate, "ids length must not be zero"),
		}
	}

	return confmanager.QueryConfigurationResponse{
		Status:         utils.BuildSuccessResponseStatus(),
		Configurations: s.GetConfs(ou, request.GetIds()),
	}
}

// GetConfs get multi confs by role.
// Params role should not be empty.
func (s *configurationService) GetConfs(role string, confIDs []string) map[string]*confmanager.QueryConfigurationResult {
	resultChannels := make([]<-chan getConfResultAsync, len(confIDs))
	for i, id := range confIDs {
		resultChannels[i] = s.GetConfAsync(role, id)
	}
	results := make(map[string]*confmanager.QueryConfigurationResult, len(resultChannels))
	for _, c := range resultChannels {
		result := <-c
		if result.err != nil {
			results[result.confID] = &confmanager.QueryConfigurationResult{
				Success: false,
				ErrMsg:  result.err.Error(),
			}
		} else {
			results[result.confID] = &confmanager.QueryConfigurationResult{
				Success: true,
				Content: result.value,
			}
		}
	}
	return results
}

// GetConfAsync get conf for role async. The role should have permission for the key.
func (s *configurationService) GetConfAsync(role string, confID string) <-chan getConfResultAsync {
	resultChan := make(chan getConfResultAsync, 0)
	go func() {
		result, err := s.GetConf(role, confID)
		resultChan <- getConfResultAsync{
			confID: confID,
			value:  result,
			err:    err,
		}
	}()
	return resultChan
}

// GetConf get conf for role. The role should have permission for the key.
func (s *configurationService) GetConf(role string, confID string) (string, error) {
	if role == "" || confID == "" {
		return "", fmt.Errorf("role or confID should not be empty")
	}
	if s.enableAuth {
		group := s.groupOf(role)
		yes, err := s.hasPermissionFor(group, confID)
		if err != nil {
			nlog.Errorf("Check permission failed for role=%s and confID=%s failed: %v", role, confID, err)
			return "", err
		}
		if !yes {
			return "", fmt.Errorf("role=%s has no permission to confID=%s", role, confID)
		}
	}
	value, err := s.backend.Get(confID)
	if err != nil {
		nlog.Errorf("Get key for confID=%s failed: %s", confID, err)
		return "", err
	}
	return value, nil
}

// hasPermissionFor check whether group has permission to access configuration.
func (s *configurationService) hasPermissionFor(group string, confID string) (bool, error) {
	// GroupKuscia has permission to access all configurations.
	if group == GroupKuscia {
		return true, nil
	}
	permissionKey := s.groupPermissionKey(group, confID)
	_, err := s.backend.Get(permissionKey)
	if err != nil {
		nlog.Errorf("Get permission key for group=%s and confID=%s failed: %s", group, confID, err)
		return false, nil
	}
	return true, nil
}

// groupOf return group which the role joined.
func (s *configurationService) groupOf(role string) string {
	return role
}

// groupPermissionKey return the permission key.
func (s *configurationService) groupPermissionKey(group string, confID string) string {
	return fmt.Sprintf("%s.%s", group, confID)
}

type getConfResultAsync struct {
	confID string
	value  string
	err    error
}
