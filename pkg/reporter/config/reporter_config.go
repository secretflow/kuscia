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

package config

import (
	"crypto/rsa"
	"path"

	"github.com/secretflow/kuscia/pkg/common"
	"k8s.io/client-go/kubernetes"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

type ReporterConfig struct {
	HTTPPort       int32                     `yaml:"HTTPPort,omitempty"`
	Debug          bool                      `yaml:"debug,omitempty"`
	ConnectTimeout int                       `yaml:"connectTimeout,omitempty"`
	ReadTimeout    int                       `yaml:"readTimeout,omitempty"`
	IdleTimeout    int                       `yaml:"idleTimeout,omitempty"`
	WriteTimeout   int                       `yaml:"-"`
	DomainKey      *rsa.PrivateKey           `yaml:"-"`
	KusciaClient   kusciaclientset.Interface `yaml:"-"`
	KubeClient     kubernetes.Interface      `yaml:"-"`
	RunMode        common.RunModeType        `yaml:"-"`
	ConfDir        string                    `yaml:"-"`
	DomainID       string                    `yaml:"-"`
}

func NewDefaultReporterConfig(rootDir string) *ReporterConfig {
	return &ReporterConfig{
		HTTPPort:       8050,
		ConnectTimeout: 5,
		ReadTimeout:    20,
		WriteTimeout:   0, // WriteTimeout must be 0 , To support http stream
		IdleTimeout:    300,
		ConfDir:        path.Join(rootDir, common.ConfPrefix),
	}
}
