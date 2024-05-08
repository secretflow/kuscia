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

package controller

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

const (
	validReqForDomainResourcePath      = "/api/v1/namespaces/alice-test-9d060f86b01acbb3/pods/secretflow-5655d668fc-g8j8j/test"
	invalidReqForDomainResourcePath    = "/api/v1/namespaces/alice-test-9d060f86b01acbb3/kusciatasks/secretflow-cdge35jv23fid2snn96g/test"
	validRefReqForDomainResourcePath   = "/api/v1/namespaces/alice-test-9d060f86b01acbb3/pods/secretflow-5655d668fc-g8j8j/test/reference"
	invalidRefReqForDomainResourcePath = "/api/v1/namespaces/alice-test-9d060f86b01acbb3/pods/secretflow-5655d668fc-g8j8j/test/reference-test"
	validReqForClusterResourcePath     = "/api/v1/kusciatasks/secretflow-cdge35jv23fid2snn96g/task-input-config"
	invalidReqForClusterResourcePath   = "/api/v1/pods/secretflow-cdge35jv23fid2snn96g/task-input-config"
)

type mockServiceBean struct {
	framework.Bean
	framework.Config

	createErr            bool
	createForRefResource bool
	findErr              bool
	deleteErr            bool
}

func (m *mockServiceBean) GetBeanByName(name string) (framework.Bean, bool) { return m, false }
func (m *mockServiceBean) GetConfigByName(name string) (framework.Config, bool) {
	return nil, false

}
func (m *mockServiceBean) Init(e framework.ConfBeanRegistry) error                       { return nil }
func (m *mockServiceBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error { return nil }

func (m *mockServiceBean) Create(resource *model.Resource, data *model.Data) error {
	if m.createErr {
		return fmt.Errorf("create failed")
	}
	return nil
}

func (m *mockServiceBean) CreateForRefResource(domain, kind, name string, resource *model.Resource) error {
	if m.createForRefResource {
		return fmt.Errorf("create for reference resource failed")
	}
	return nil
}

func (m *mockServiceBean) Find(domain, kind, name string) (*model.Data, error) {
	if m.findErr {
		return nil, fmt.Errorf("find failed")
	}
	return &model.Data{
		Content: "mock test",
	}, nil
}

func (m *mockServiceBean) Delete(domain, kind, name string) error {
	if m.deleteErr {
		return fmt.Errorf("delete failed")
	}
	return nil
}

func setGinParams(ctx *gin.Context, params map[string]string) {
	var ginParams gin.Params
	for key, value := range params {
		param := gin.Param{
			Key:   key,
			Value: value,
		}
		ginParams = append(ginParams, param)
	}
	ctx.Params = ginParams
}

func setHTTPRequest(ctx *gin.Context, method, path, body string) {
	if body == "" {
		ctx.Request = httptest.NewRequest(method, path, nil)
		return
	}
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
}

func TestIsDomainResourceRequest(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "request path is for domain resource",
			path:     validReqForDomainResourcePath,
			expected: true,
		},
		{
			name:     "request path isn't for domain resource",
			path:     validReqForClusterResourcePath,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := isDomainResourceRequest(tc.path)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestIsReferenceRequest(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "invalid reference request",
			path:     invalidRefReqForDomainResourcePath,
			expected: false,
		},
		{
			name:     "reference request",
			path:     validRefReqForDomainResourcePath,
			expected: true,
		},
		{
			name:     "normal request",
			path:     validReqForDomainResourcePath,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := isReferenceRequest(tc.path)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestValidateRequestKind(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		kind        string
		expectedErr bool
	}{
		{
			name:        "valid domain resource kind",
			path:        validReqForDomainResourcePath,
			kind:        "pods",
			expectedErr: false,
		},
		{
			name:        "invalid domain resource kind",
			path:        invalidReqForDomainResourcePath,
			kind:        "kusciatasks",
			expectedErr: true,
		},
		{
			name:        "valid cluster resource kind",
			path:        validReqForClusterResourcePath,
			kind:        "kusciatasks",
			expectedErr: false,
		},
		{
			name:        "invalid cluster resource kind",
			path:        invalidReqForClusterResourcePath,
			kind:        "pods",
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRequestKind(tc.kind, tc.path)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
