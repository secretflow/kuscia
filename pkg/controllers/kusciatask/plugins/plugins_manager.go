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

//nolint:dupl
package plugins

import (
	"context"
)

type ResourceRequest struct {
	DomainName string
	CpuReq     int64
	MemReq     int64
}

type Plugin interface {
	Permit(ctx context.Context, params interface{}) (bool, error)
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]Plugin, 0),
	}
}

func (pm *PluginManager) Register(p Plugin) {
	pm.plugins = append(pm.plugins, p)
}

func (pm *PluginManager) Permit(ctx context.Context, params interface{}) (bool, []error) {
	var errors []error
	for _, p := range pm.plugins {
		_, err := p.Permit(ctx, params)
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}
	return len(errors) == 0, errors
}
