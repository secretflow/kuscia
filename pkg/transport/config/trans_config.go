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

package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/transport/msq"
)

type TransConfig struct {
	MsqConfig  *msq.Config   `yaml:"msqConfig,omitempty"`
	HTTPConfig *ServerConfig `yaml:"httpConfig,omitempty"`
}

func LoadTransConfig(configPath string) (*TransConfig, error) {
	return LoadOverrideConfig(DefaultTransConfig(), configPath)
}

func DefaultTransConfig() *TransConfig {
	return &TransConfig{
		MsqConfig:  msq.DefaultMsgConfig(),
		HTTPConfig: DefaultServerConfig(),
	}
}

func LoadOverrideConfig(config *TransConfig, configPath string) (*TransConfig, error) {
	if configPath == "" {
		return config, nil // no need to load config file
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, config)
	return config, err
}

func (c *TransConfig) Check() error {
	if err := c.MsqConfig.Check(); err != nil {
		return err
	}
	return nil
}
