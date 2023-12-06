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
	"crypto/rsa"
	"fmt"
	"sync/atomic"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

type ConfManagerConfig struct {
	HTTPPort       int32  `yaml:"HTTPPort,omitempty"`
	GRPCPort       int32  `yaml:"GRPCPort,omitempty"`
	ConnectTimeout int    `yaml:"connectTimeout,omitempty"`
	ReadTimeout    int    `yaml:"readTimeout,omitempty"`
	WriteTimeout   int    `yaml:"writeTimeout,omitempty"`
	IdleTimeout    int    `yaml:"idleTimeout,omitempty"`
	EnableConfAuth bool   `yaml:"enableConfAuth,omitempty"`
	Backend        string `yaml:"backend,omitempty"`

	DomainID        string                     `yaml:"-"`
	DomainKey       *rsa.PrivateKey            `yaml:"-"`
	BackendDriver   secretbackend.SecretDriver `yaml:"-"`
	TLS             config.TLSServerConfig     `yaml:"-"`
	DomainCertValue *atomic.Value              `yaml:"-"`
	IsMaster        bool                       `yaml:"-"`
}

type SAN struct {
	DNSNames []string `yaml:"dnsNames"`
	IPs      []string `yaml:"ips"`
}

func NewDefaultConfManagerConfig() *ConfManagerConfig {
	return &ConfManagerConfig{
		HTTPPort:       8060,
		GRPCPort:       8061,
		ConnectTimeout: 5,
		ReadTimeout:    20,
		WriteTimeout:   20,
		IdleTimeout:    300,
		Backend:        common.DefaultSecretBackendName,
		IsMaster:       false,
	}
}

func (c *ConfManagerConfig) MustTLSEnables(errs *errorcode.Errs) {
	if c.TLS.RootCA == nil {
		errs.AppendErr(fmt.Errorf("for confmanager, tls root ca should not be empty"))
	}
	if c.TLS.ServerCert == nil {
		errs.AppendErr(fmt.Errorf("for confmanager, tls server cert should not be empty"))
	}
	if c.TLS.ServerKey == nil {
		errs.AppendErr(fmt.Errorf("for confmanager, tls server key should not be empty"))
	}
}
