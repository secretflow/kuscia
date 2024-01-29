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

const (
	defaultGrpcPort             = 9090
	defaultMaxConns             = 32
	defaultMaxConcurrentStreams = 128
	defaultMaxRecvMsgSize       = 4194304
	defaultMaxSendMsgSize       = 4194304
	defaultConnectionTimeout    = 120
	defaultReadBufferSize       = 32768
	defaultWriteBufferSize      = 32768
)

type GrpcConfig struct {
	Port                 int    `yaml:"port,omitempty"`
	MaxConns             int    `yaml:"maxConns,omitempty"`
	MaxConcurrentStreams uint32 `yaml:"maxConcurrentStreams,omitempty"`
	MaxRecvMsgSize       int    `yaml:"maxRecvMsgSize,omitempty"`
	MaxSendMsgSize       int    `yaml:"maxSendMsgSize,omitempty"`
	ConnectionTimeout    uint32 `yaml:"connectionTimeout,omitempty"`
	ReadBufferSize       int    `yaml:"readBufferSize,omitempty"`
	WriteBufferSize      int    `yaml:"writeBufferSize,omitempty"`
}

func DefaultGrpcConfig() *GrpcConfig {
	return &GrpcConfig{
		Port:                 defaultGrpcPort,
		MaxConns:             defaultMaxConns,
		MaxConcurrentStreams: defaultMaxConcurrentStreams,
		MaxRecvMsgSize:       defaultMaxRecvMsgSize,
		MaxSendMsgSize:       defaultMaxSendMsgSize,
		ConnectionTimeout:    defaultConnectionTimeout,
		ReadBufferSize:       defaultReadBufferSize,
		WriteBufferSize:      defaultWriteBufferSize,
	}
}

type GrpcTransConfig struct {
	MsqConfig  *msq.Config `yaml:"msqConfig,omitempty"`
	GrpcConfig *GrpcConfig `yaml:"grpcConfig,omitempty"`
}

func DefaultGrpcTransConfig() *GrpcTransConfig {
	return &GrpcTransConfig{
		MsqConfig:  msq.DefaultMsgConfig(),
		GrpcConfig: DefaultGrpcConfig(),
	}
}

func LoadOverrideGrpcTransConfig(config *GrpcTransConfig, configPath string) (*GrpcTransConfig, error) {
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

func (c *GrpcTransConfig) Check() error {
	if err := c.MsqConfig.Check(); err != nil {
		return err
	}
	return nil
}
