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
	"path"

	"k8s.io/client-go/kubernetes"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
)

type KusciaAPIConfig struct {
	HTTPPort       int32
	GRPCPort       int32
	Debug          bool
	ConnectTimeOut int64
	ReadTimeout    int64
	WriteTimeout   int64
	IdleTimeout    int64
	Initiator      string
	DomainKeyFile  string
	TLSConfig      *TLSConfig
	TokenConfig    *TokenConfig
	KusciaClient   kusciaclientset.Interface
	KubeClient     kubernetes.Interface
}

type TLSConfig struct {
	RootCACertFile string
	ServerCertFile string
	ServerKeyFile  string
}

type TokenConfig struct {
	TokenFile string
}

func NewDefaultKusciaAPIConfig(rootDir string) *KusciaAPIConfig {
	return &KusciaAPIConfig{
		HTTPPort:       8082,
		GRPCPort:       8083,
		ConnectTimeOut: 5,
		ReadTimeout:    20,
		WriteTimeout:   20,
		IdleTimeout:    300,
		TLSConfig: &TLSConfig{
			RootCACertFile: path.Join(rootDir, constants.CertPathPrefix, "ca.crt"),
			ServerKeyFile:  path.Join(rootDir, constants.CertPathPrefix, "kusciaapi-server.key"),
			ServerCertFile: path.Join(rootDir, constants.CertPathPrefix, "kusciaapi-server.crt"),
		},
		TokenConfig: &TokenConfig{
			TokenFile: path.Join(rootDir, constants.CertPathPrefix, "token"),
		},
	}
}
