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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

type mockRegistry struct {
	framework.Bean
	framework.Config

	createErr               bool
	createForRefResourceErr bool
	findErr                 bool
	deleteErr               bool
}

func (m mockRegistry) Init(e framework.ConfBeanRegistry) error                       { return nil }
func (m mockRegistry) Start(ctx context.Context, e framework.ConfBeanRegistry) error { return nil }
func (m mockRegistry) GetBeanByName(name string) (framework.Bean, bool)              { return m, true }
func (m mockRegistry) GetConfigByName(name string) (framework.Config, bool)          { return nil, false }

func (m mockRegistry) Create(resource *model.Resource, data *model.Data) error {
	if m.createErr {
		return fmt.Errorf("create failed")
	}
	return nil
}

func (m mockRegistry) CreateForRefResource(domain, kind, name string, resource *model.Resource) error {
	if m.createForRefResourceErr {
		return fmt.Errorf("create for referece resource failed")
	}
	return nil
}

func (m mockRegistry) Find(domain, kind, name string) (*model.Data, error) {
	if m.findErr {
		return nil, fmt.Errorf("find failed")
	}
	return &model.Data{Content: "test"}, nil
}

func (m mockRegistry) Delete(domain, kind, name string) error {
	if m.deleteErr {
		return fmt.Errorf("delete failed")
	}
	return nil
}

func TestResourceServiceInit(t *testing.T) {
	testCases := []struct {
		name        string
		rs          IService
		registry    framework.ConfBeanRegistry
		expectedErr bool
	}{
		{
			name:        "init success",
			rs:          NewResourceService(),
			registry:    mockRegistry{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rs.Init(tc.registry)
			t.Logf("err: %v", err)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestResourceServiceCreate(t *testing.T) {
	id := uint(1)
	resource := &model.Resource{
		Domain: "alice-test-9d060f86b01acbb3",
		Kind:   "pods",
		Name:   "secretflow-5655d668fc-g8j8j/test",
		DataID: &id,
	}

	data := &model.Data{ID: id, Content: "test"}

	testCases := []struct {
		name        string
		rs          IService
		registry    framework.ConfBeanRegistry
		resource    *model.Resource
		data        *model.Data
		expectedErr bool
	}{
		{
			name: "create failed",
			rs:   NewResourceService(),
			registry: mockRegistry{
				createErr: true,
			},
			expectedErr: true,
		},
		{
			name:        "create success",
			rs:          NewResourceService(),
			registry:    mockRegistry{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = tc.rs.Init(tc.registry)
			err := tc.rs.Create(resource, data)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestResourceServiceCreateForRefResource(t *testing.T) {
	var (
		domain = "alice-test-9d060f86b01acbb3"
		kind   = "pods"
		name   = "secretflow-5655d668fc-g8j8j/test"
	)

	testCases := []struct {
		name        string
		rs          IService
		registry    framework.ConfBeanRegistry
		expectedErr bool
	}{
		{
			name: "create failed",
			rs:   NewResourceService(),
			registry: mockRegistry{
				createForRefResourceErr: true,
			},
			expectedErr: true,
		},
		{
			name:        "create success",
			rs:          NewResourceService(),
			registry:    mockRegistry{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = tc.rs.Init(tc.registry)
			err := tc.rs.CreateForRefResource(domain, kind, name, nil)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestResourceServiceFind(t *testing.T) {
	var (
		domain = "alice-test-9d060f86b01acbb3"
		kind   = "pods"
		name   = "secretflow-5655d668fc-g8j8j/test"
	)

	testCases := []struct {
		name        string
		rs          IService
		registry    framework.ConfBeanRegistry
		expectedErr bool
	}{
		{
			name: "find failed",
			rs:   NewResourceService(),
			registry: mockRegistry{
				findErr: true,
			},
			expectedErr: true,
		},
		{
			name:        "find success",
			rs:          NewResourceService(),
			registry:    mockRegistry{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = tc.rs.Init(tc.registry)
			_, err := tc.rs.Find(domain, kind, name)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestResourceServiceDelete(t *testing.T) {
	var (
		domain = "alice-test-9d060f86b01acbb3"
		kind   = "pods"
		name   = "secretflow-5655d668fc-g8j8j/test"
	)

	testCases := []struct {
		name        string
		rs          IService
		registry    framework.ConfBeanRegistry
		expectedErr bool
	}{
		{
			name: "find failed",
			rs:   NewResourceService(),
			registry: mockRegistry{
				deleteErr: true,
			},
			expectedErr: true,
		},
		{
			name:        "find success",
			rs:          NewResourceService(),
			registry:    mockRegistry{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = tc.rs.Init(tc.registry)
			err := tc.rs.Delete(domain, kind, name)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
