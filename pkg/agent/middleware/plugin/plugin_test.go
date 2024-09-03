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

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

type mockPluginA struct{}

const (
	mockPluginType = "mock_type"
)

func (p *mockPluginA) Type() string {
	return mockPluginType
}

func (p *mockPluginA) Init(ctx context.Context, dependencies *Dependencies, cfg *config.PluginCfg) error {
	return nil
}

type mockPluginService struct{}

func (p *mockPluginService) Type() string {
	return mockPluginType
}

func (p *mockPluginService) Init(ctx context.Context, dependencies *Dependencies, cfg *config.PluginCfg) error {
	return nil
}

func TestRegisterGet(t *testing.T) {
	Register("mock_plugin_a", &mockPluginA{})

	f := Get("mock_plugin_a")
	assert.True(t, f != nil)

	f = Get("mock_plugin_0")
	assert.True(t, f == nil)
}

func TestInit(t *testing.T) {
	t.Run("setup succeed", func(t *testing.T) {
		const configYaml = `
plugins:
  - name: mock_plugin_a
    config: ====
`
		agentConfig := config.DefaultAgentConfig()
		assert.NoError(t, yaml.Unmarshal([]byte(configYaml), &agentConfig))
		dep := &Dependencies{AgentConfig: agentConfig}

		Register("mock_plugin_a", &mockPluginA{})

		assert.NoError(t, Init(context.Background(), dep))
	})

	t.Run("setup failed", func(t *testing.T) {
		const configYaml = `
plugins:
  - name: mock_plugin_0
    config: ====
`
		agentConfig := config.DefaultAgentConfig()
		assert.NoError(t, yaml.Unmarshal([]byte(configYaml), &agentConfig))
		dep := &Dependencies{AgentConfig: agentConfig}

		assert.ErrorContains(t, Init(context.Background(), dep), "not registered")
	})

}
