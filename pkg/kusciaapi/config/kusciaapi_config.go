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
	"crypto/x509"
	"path"
	"sync/atomic"

	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

type KusciaAPIConfig struct {
	HTTPPort         int32                     `yaml:"HTTPPort,omitempty"`
	HTTPInternalPort int32                     `yaml:"HTTPInternalPort,omitempty"`
	GRPCPort         int32                     `yaml:"GRPCPort,omitempty"`
	Debug            bool                      `yaml:"debug,omitempty"`
	ConnectTimeout   int                       `yaml:"connectTimeout,omitempty"`
	ReadTimeout      int                       `yaml:"readTimeout,omitempty"`
	IdleTimeout      int                       `yaml:"idleTimeout,omitempty"`
	Initiator        string                    `yaml:"initiator,omitempty"`
	Protocol         common.Protocol           `yaml:"protocol"`
	Token            *TokenConfig              `yaml:"token"`
	WriteTimeout     int                       `yaml:"-"`
	TLS              *config.TLSServerConfig   `yaml:"-"`
	DomainKey        *rsa.PrivateKey           `yaml:"-"`
	RootCAKey        *rsa.PrivateKey           `yaml:"-"`
	RootCA           *x509.Certificate         `yaml:"-"`
	KusciaClient     kusciaclientset.Interface `yaml:"-"`
	KubeClient       kubernetes.Interface      `yaml:"-"`
	RunMode          common.RunModeType        `yaml:"-"`
	DomainCertValue  *atomic.Value             `yaml:"-"`
	ConfDir          string                    `yaml:"-"`
	DomainID         string                    `yaml:"-"`
}

type TokenConfig struct {
	TokenFile string
}

func NewDefaultKusciaAPIConfig(rootDir string) *KusciaAPIConfig {
	return &KusciaAPIConfig{
		HTTPPort: 8082,
		// TODO LLY-UMF 默认端口配置
		GRPCPort:         8083,
		HTTPInternalPort: 8092,
		ConnectTimeout:   5,
		ReadTimeout:      20,
		WriteTimeout:     0, // WriteTimeout must be 0 , To support http stream
		IdleTimeout:      300,
		TLS: &config.TLSServerConfig{
			ServerKeyFile:  path.Join(rootDir, common.CertPrefix, "kusciaapi-server.key"),
			ServerCertFile: path.Join(rootDir, common.CertPrefix, "kusciaapi-server.crt"),
		},
		Token: &TokenConfig{
			TokenFile: path.Join(rootDir, common.CertPrefix, "token"),
		},
		ConfDir: path.Join(rootDir, common.ConfPrefix),
	}
}
