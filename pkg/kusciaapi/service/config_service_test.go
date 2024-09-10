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
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/driver"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func makeCMConfigService() (cmservice.IConfigService, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	encValue, err := tls.EncryptOAEP(&privateKey.PublicKey, []byte("test-value"))
	if err != nil {
		return nil, err
	}
	configmap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "alice",
			Name:      "domain-config",
		},
		Data: map[string]string{
			"test-key": encValue,
		},
	}
	kubeClient := kubefake.NewSimpleClientset(&configmap)
	return cmservice.NewConfigService(context.Background(), &cmservice.ConfigServiceConfig{
		DomainID:     "alice",
		DomainKey:    privateKey,
		Driver:       driver.CRDDriverType,
		DisableCache: true,
		KubeClient:   kubeClient,
	})
}

func mockConfigService(t *testing.T, runMode string) IConfigService {
	conf := &config.KusciaAPIConfig{
		RunMode:  runMode,
		DomainID: "alice",
	}
	cmConfigService, err := makeCMConfigService()
	assert.NoError(t, err)
	return NewConfigService(conf, cmConfigService)
}

func TestCreateConfig(t *testing.T) {
	tests := []struct {
		name     string
		runMode  string
		req      *kusciaapi.CreateConfigRequest
		wantCode errorcode.ErrorCode
	}{
		{
			name:     "request data is empty, should return error",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.CreateConfigRequest{Data: []*kusciaapi.ConfigData{}},
			wantCode: errorcode.ErrorCode_KusciaAPIErrRequestValidate,
		},
		{
			name:    "request is valid, should return success",
			runMode: common.RunModeLite,
			req: &kusciaapi.CreateConfigRequest{Data: []*kusciaapi.ConfigData{
				{
					Key:   "key",
					Value: "value",
				},
			}},
			wantCode: errorcode.ErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mockConfigService(t, tt.runMode)
			got := svc.CreateConfig(context.Background(), tt.req)
			assert.Equal(t, int32(tt.wantCode), got.Status.Code)
		})
	}
}

func TestQueryConfig(t *testing.T) {
	tests := []struct {
		name     string
		runMode  string
		req      *kusciaapi.QueryConfigRequest
		wantCode errorcode.ErrorCode
	}{
		{
			name:     "request key is empty, should return error",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.QueryConfigRequest{Key: ""},
			wantCode: errorcode.ErrorCode_KusciaAPIErrRequestValidate,
		},
		{
			name:     "request is valid, should return success",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.QueryConfigRequest{Key: "test-key"},
			wantCode: errorcode.ErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mockConfigService(t, tt.runMode)
			got := svc.QueryConfig(context.Background(), tt.req)
			assert.Equal(t, int32(tt.wantCode), got.Status.Code)
		})
	}
}

func TestBatchQueryConfig(t *testing.T) {
	tests := []struct {
		name     string
		runMode  string
		req      *kusciaapi.BatchQueryConfigRequest
		wantCode errorcode.ErrorCode
	}{
		{
			name:    "request keys is empty, should return error",
			runMode: common.RunModeLite,
			req: &kusciaapi.BatchQueryConfigRequest{
				Keys: nil,
			},
			wantCode: errorcode.ErrorCode_KusciaAPIErrRequestValidate,
		},
		{
			name:    "request is valid, should return success",
			runMode: common.RunModeLite,
			req: &kusciaapi.BatchQueryConfigRequest{
				Keys: []string{"test-key"},
			},
			wantCode: errorcode.ErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mockConfigService(t, tt.runMode)
			got := svc.BatchQueryConfig(context.Background(), tt.req)
			assert.Equal(t, int32(tt.wantCode), got.Status.Code)
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	tests := []struct {
		name     string
		runMode  string
		req      *kusciaapi.UpdateConfigRequest
		wantCode errorcode.ErrorCode
	}{
		{
			name:     "request data is empty, should return error",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.UpdateConfigRequest{},
			wantCode: errorcode.ErrorCode_KusciaAPIErrRequestValidate,
		},
		{
			name:     "request is valid, should return success",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.UpdateConfigRequest{Data: []*kusciaapi.ConfigData{{Key: "key", Value: "value"}}},
			wantCode: errorcode.ErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mockConfigService(t, tt.runMode)
			got := svc.UpdateConfig(context.Background(), tt.req)
			assert.Equal(t, int32(tt.wantCode), got.Status.Code)
		})
	}
}

func TestDeleteConfig(t *testing.T) {
	tests := []struct {
		name     string
		runMode  string
		req      *kusciaapi.DeleteConfigRequest
		wantCode errorcode.ErrorCode
	}{
		{
			name:     "request key is empty, should return error",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.DeleteConfigRequest{},
			wantCode: errorcode.ErrorCode_KusciaAPIErrRequestValidate,
		},
		{
			name:     "request is valid, should return success",
			runMode:  common.RunModeLite,
			req:      &kusciaapi.DeleteConfigRequest{Keys: []string{"key"}},
			wantCode: errorcode.ErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mockConfigService(t, tt.runMode)
			got := svc.DeleteConfig(context.Background(), tt.req)
			assert.Equal(t, int32(tt.wantCode), got.Status.Code)
		})
	}
}
