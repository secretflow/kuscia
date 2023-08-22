package config

import (
	"github.com/secretflow/kuscia/pkg/transport/msq"
	"gopkg.in/yaml.v3"
	"os"
)

type GRPCConfig struct {
	Port                 int    `yaml:"port,omitempty"`
	MaxConns             int    `yaml:"maxConns,omitempty"`
	MaxConcurrentStreams uint32 `yaml:"maxConcurrentStreams,omitempty"`
	MaxReadFrameSize     int    `yaml:"maxReadFrameSize,omitempty"`
}

func DefaultGrpcConfig() *GRPCConfig {
	return &GRPCConfig{
		Port:                 9090,
		MaxConns:             32,
		MaxConcurrentStreams: 128,
		MaxReadFrameSize:     134217728,
	}
}

type GrpcTransConfig struct {
	MsqConfig  *msq.Config `yaml:"msqConfig,omitempty"`
	GrpcConfig *GRPCConfig `yaml:"grpcConfig,omitempty"`
}

func DefaultGrpcTransConfig() *GrpcTransConfig {
	return &GrpcTransConfig{
		MsqConfig:  msq.DefaultMsgConfig(),
		GrpcConfig: DefaultGrpcConfig(),
	}
}

func LoadOverrideGrpcTransConfig(config *TransConfig, configPath string) (*TransConfig, error) {
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
