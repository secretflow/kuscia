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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/driver"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

func makeConfigService() (IConfigService, error) {
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
			Name:      config.DomainConfigName,
		},
		Data: map[string]string{
			"test-key": encValue,
		},
	}
	kubeClient := kubefake.NewSimpleClientset(&configmap)

	return NewConfigService(context.Background(), &ConfigServiceConfig{
		DomainID:     "alice",
		DomainKey:    privateKey,
		Driver:       driver.CRDDriverType,
		DisableCache: true,
		KubeClient:   kubeClient,
	})
}

func Test_ConfigService_CreateConfig(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.CreateConfigRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "request data is empty, should return error",
			args: args{
				ctx:     context.Background(),
				request: &confmanager.CreateConfigRequest{},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request data key is empty, should return error",
			args: args{
				ctx: context.Background(),
				request: &confmanager.CreateConfigRequest{
					Data: []*confmanager.ConfigData{
						{
							Key:   "",
							Value: "value",
						},
					},
				},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request is valid",
			args: args{
				ctx: context.Background(),
				request: &confmanager.CreateConfigRequest{
					Data: []*confmanager.ConfigData{
						{
							Key:   "key",
							Value: "value",
						},
					},
				},
			},
			wantCode: int32(errorcode.ErrorCode_SUCCESS),
		},
	}
	s, err := makeConfigService()
	assert.Nil(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.CreateConfig(tt.args.ctx, tt.args.request)
			assert.Equal(t, tt.wantCode, got.Status.Code)
		})
	}
}

func Test_ConfigService_QueryConfig(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.QueryConfigRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "request key is empty",
			args: args{
				ctx: context.Background(),
				request: &confmanager.QueryConfigRequest{
					Key: "",
				},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request is valid",
			args: args{
				ctx: context.Background(),
				request: &confmanager.QueryConfigRequest{
					Key: "test-key",
				},
			},
			wantCode: int32(errorcode.ErrorCode_SUCCESS),
		},
	}
	s, err := makeConfigService()
	assert.Nil(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.QueryConfig(tt.args.ctx, tt.args.request)
			assert.Equal(t, tt.wantCode, got.Status.Code)
		})
	}
}

func Test_ConfigService_UpdateConfig(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.UpdateConfigRequest
	}
	tests := []struct {
		name      string
		args      args
		wantCode  int32
		wantValue string
	}{
		{
			name: "request data is empty",
			args: args{
				ctx:     context.Background(),
				request: &confmanager.UpdateConfigRequest{},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request data key is empty",
			args: args{
				ctx: context.Background(),
				request: &confmanager.UpdateConfigRequest{
					Data: []*confmanager.ConfigData{
						{
							Key:   "",
							Value: "value",
						},
					},
				},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request is valid",
			args: args{
				ctx: context.Background(),
				request: &confmanager.UpdateConfigRequest{
					Data: []*confmanager.ConfigData{
						{
							Key:   "test-key",
							Value: "value",
						},
					},
				},
			},
			wantCode:  int32(errorcode.ErrorCode_SUCCESS),
			wantValue: "value",
		},
	}
	s, err := makeConfigService()
	assert.Nil(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.UpdateConfig(tt.args.ctx, tt.args.request)
			assert.Equal(t, tt.wantCode, got.Status.Code)
			if tt.wantValue != "" {
				res := s.QueryConfig(tt.args.ctx, &confmanager.QueryConfigRequest{
					Key: "test-key",
				})
				assert.Equal(t, tt.wantValue, res.Value)
			}
		})
	}
}

func Test_ConfigService_DeleteConfig(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.DeleteConfigRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "request keys is empty",
			args: args{
				ctx:     context.Background(),
				request: &confmanager.DeleteConfigRequest{},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request is valid",
			args: args{
				ctx: context.Background(),
				request: &confmanager.DeleteConfigRequest{
					Keys: []string{"test"}},
			},
			wantCode: int32(errorcode.ErrorCode_SUCCESS),
		},
	}
	s, err := makeConfigService()
	assert.Nil(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.DeleteConfig(tt.args.ctx, tt.args.request)
			assert.Equal(t, tt.wantCode, got.Status.Code)
		})
	}
}

func Test_ConfigService_BatchQueryConfig(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.BatchQueryConfigRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "request keys is empty",
			args: args{
				ctx:     context.Background(),
				request: &confmanager.BatchQueryConfigRequest{},
			},
			wantCode: int32(errorcode.ErrorCode_ConfManagerErrRequestInvalidate),
		},
		{
			name: "request is valid",
			args: args{
				ctx: context.Background(),
				request: &confmanager.BatchQueryConfigRequest{
					Keys: []string{"test-key"},
				},
			},
			wantCode: int32(errorcode.ErrorCode_SUCCESS),
		},
	}
	s, err := makeConfigService()
	assert.Nil(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.BatchQueryConfig(tt.args.ctx, tt.args.request)
			assert.Equal(t, tt.wantCode, got.Status.Code)
		})
	}
}
