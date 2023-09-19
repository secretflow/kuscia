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
	"fmt"
	"path"

	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

type ConfManagerConfig struct {
	HTTPPort       int32                `yaml:"HTTPPort,omitempty"`
	GRPCPort       int32                `yaml:"GRPCPort,omitempty"`
	ConnectTimeout int                  `yaml:"connectTimeout,omitempty"`
	ReadTimeout    int                  `yaml:"readTimeout,omitempty"`
	WriteTimeout   int                  `yaml:"writeTimeout,omitempty"`
	IdleTimeout    int                  `yaml:"idleTimeout,omitempty"`
	TLSConfig      *TLSConfig           `yaml:"tls,omitempty"`
	EnableConfAuth bool                 `yaml:"enableConfAuth,omitempty"`
	SecretBackend  *SecretBackendConfig `yaml:"backend,omitempty"`
}

type TLSConfig struct {
	RootCAFile     string `yaml:"rootCAFile"`
	ServerCertFile string `yaml:"serverCertFile"`
	ServerKeyFile  string `yaml:"serverKeyFile"`
}

type SecretBackendConfig struct {
	Driver string         `yaml:"driver"`
	Params map[string]any `yaml:"params"`
}

func NewDefaultConfManagerConfig(rootDir string) *ConfManagerConfig {
	return &ConfManagerConfig{
		HTTPPort:       8060,
		GRPCPort:       8061,
		ConnectTimeout: 5,
		ReadTimeout:    20,
		WriteTimeout:   20,
		IdleTimeout:    300,
		TLSConfig: &TLSConfig{
			RootCAFile:     path.Join(rootDir, constants.CertPathPrefix, "ca.crt"),
			ServerKeyFile:  path.Join(rootDir, constants.CertPathPrefix, "confmanager-server.key"),
			ServerCertFile: path.Join(rootDir, constants.CertPathPrefix, "confmanager-server.crt"),
		},
		EnableConfAuth: false,
		SecretBackend: &SecretBackendConfig{
			Driver: "mem",
			Params: map[string]any{},
		},
	}
}

func (c ConfManagerConfig) MustTLSEnables(errs *errorcode.Errs) {
	if c.TLSConfig == nil {
		errs.AppendErr(fmt.Errorf("for confmanager, grpc must use tls"))
	}
	if c.TLSConfig.RootCAFile == "" {
		errs.AppendErr(fmt.Errorf("for confmanager, grpc tls root ca file should not be empty"))
	}
	if c.TLSConfig.ServerCertFile == "" {
		errs.AppendErr(fmt.Errorf("for confmanager, grpc tls server cert file should not be empty"))
	}
	if c.TLSConfig.ServerKeyFile == "" {
		errs.AppendErr(fmt.Errorf("for confmanager, grpc tls server key file should not be empty"))
	}
}
