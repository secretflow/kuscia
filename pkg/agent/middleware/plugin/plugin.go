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
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	plugins = make(map[string]Plugin) // plugin type => { plugin name => plugin factory }
)

// Plugin is a unified abstraction of plugin. External plugins need to implement this interface.
type Plugin interface {
	Type() string
	Init(context.Context, *Dependencies, *config.PluginCfg) error
}

// Register plugin.
func Register(name string, p Plugin) {
	plugins[name] = p
}

// Get plugin.
func Get(name string) Plugin {
	return plugins[name]
}

type Dependencies struct {
	AgentConfig *config.AgentConfig
	KubeClient  kubernetes.Interface
}

// Init build and load specific plugins through configuration.
// config contain the configuration of all plugins, including plugin loading configuration
// and plugin custom configuration.
// e.g.:
//
//	plugins:
//	- name: registry-filter
//	  # plugin custom configuration
//	  config:
//	  - permission: allow
//	    patterns:
//	    - docker.io/secretflow
func Init(ctx context.Context, dependencies *Dependencies) error {
	for _, pluginCfg := range dependencies.AgentConfig.Plugins {
		plugin := Get(pluginCfg.Name)
		if plugin == nil {
			return fmt.Errorf("plugin %v is not registered", pluginCfg.Name)
		}

		if err := plugin.Init(ctx, dependencies, &pluginCfg); err != nil {
			return fmt.Errorf("setup plugin %v:%v failed, detail-> %v", plugin.Type(), pluginCfg.Name, err)
		}

		nlog.Infof("Init plugin %v:%v succeed", plugin.Type(), pluginCfg.Name)
	}

	return nil
}
