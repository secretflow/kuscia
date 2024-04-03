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

package kusciaconfig

import (
	"fmt"
)

type ServiceConfig struct {
	Endpoint  string     `yaml:"endpoint,omitempty"`
	TLSConfig *TLSConfig `yaml:"tls,omitempty"`
}

type APIServerConfig struct {
	KubeConfig string `yaml:"kubeconfig,omitempty"`
	Endpoint   string `yaml:"endpoint,omitempty"`
}

type MasterConfig struct {
	ServiceConfig     `yaml:",inline"`
	DatastoreEndpoint string           `yaml:"datastoreEndpoint,omitempty"`
	ClusterToken      string           `yaml:"clusterToken"`
	APIServer         *APIServerConfig `yaml:"apiserver,omitempty"`
	KusciaStorage     *ServiceConfig   `yaml:"kusciaStorage,omitempty"`
	KusciaAPI         *ServiceConfig   `yaml:"kusciaAPI,omitempty"`
	APIWhitelist      []string         `yaml:"apiWhitelist,omitempty"`
}

func (m *MasterConfig) IsMaster() bool {
	return m.APIServer != nil
}

func CheckServiceConfig(config *ServiceConfig, name string) error {
	if config.Endpoint == "" {
		return fmt.Errorf("serviceConfig(%s) need specify endpoint", name)
	}

	if config.TLSConfig != nil {
		return CheckTLSConfig(config.TLSConfig, name)
	}

	return nil
}

func CheckAPIServerConfig(config *APIServerConfig) error {
	if config.KubeConfig == "" {
		return fmt.Errorf("apiserver must specify kubeconfig")
	}
	return nil
}

func CheckMasterConfig(config *MasterConfig) error {
	if config == nil {
		return fmt.Errorf("master config is nil")
	}

	if config.APIServer == nil && config.Endpoint == "" {
		return fmt.Errorf("master config must specify endpoint or apiserver")
	}

	if config.APIServer != nil {
		if err := CheckAPIServerConfig(config.APIServer); err != nil {
			return err
		}
	}

	if config.KusciaStorage != nil {
		return CheckServiceConfig(config.KusciaStorage, "kusciastorage")
	}

	return nil
}
