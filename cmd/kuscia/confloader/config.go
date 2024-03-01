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

package confloader

import (
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	cmconf "github.com/secretflow/kuscia/pkg/confmanager/config"
	dmconfig "github.com/secretflow/kuscia/pkg/datamesh/config"
	kaconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	defaultRootDir            = "/home/kuscia/"
	defaultEndpointForMaster  = "https://127.0.0.1:6443"
	defaultMetricUpdatePeriod = uint(5)
)

type KusciaConfig struct {
	RootDir  string `yaml:"rootDir,omitempty"`
	DomainID string `yaml:"domainID,omitempty"`

	DomainKeyFile  string `yaml:"domainKeyFile,omitempty"`
	DomainKeyData  string `yaml:"domainKeyData,omitempty"`
	DomainCertFile string `yaml:"domainCertFile,omitempty"`
	DomainCertData string `yaml:"domainCertData,omitempty"`
	CAKeyFile      string `yaml:"caKeyFile,omitempty"`
	CAKeyData      string `yaml:"caKeyData,omitempty"`
	CACertFile     string `yaml:"caFile,omitempty"` // Note: for ca cert will be mounted to agent pod
	CACertData     string `yaml:"caCertData,omitempty"`

	LogLevel           string          `yaml:"logLevel"`
	Logrotate          LogrotateConfig `yaml:"logrotate,omitempty"`
	MetricUpdatePeriod uint            `yaml:"metricUpdatePeriod,omitempty"` // Unit: second

	Debug        bool `yaml:"debug"`
	DebugPort    int  `yaml:"debugPort"`
	CtrDebugPort int  `yaml:"controllerDebugPort"`

	Agent                 config.AgentConfig        `yaml:"agent,omitempty"`
	Master                kusciaconfig.MasterConfig `yaml:"master,omitempty"`
	ConfManager           cmconf.ConfManagerConfig  `yaml:"confManager,omitempty"`
	KusciaAPI             *kaconfig.KusciaAPIConfig `yaml:"kusciaAPI,omitempty"`
	SecretBackends        []SecretBackendConfig     `yaml:"secretBackends,omitempty"`
	ConfLoaders           []ConfigLoaderConfig      `yaml:"confLoaders,omitempty"`
	DataMesh              *dmconfig.DataMeshConfig  `yaml:"dataMesh,omitempty"`
	DomainRoute           DomainRouteConfig         `yaml:"domainRoute,omitempty"`
	Protocol              common.Protocol           `yaml:"protocol"`
	EnvoyIP               string                    `yaml:"-"`
	CoreDNSBackUpConf     string                    `yaml:"-"`
	RunMode               common.RunModeType        `yaml:"-"`
	EnableWorkloadApprove bool                      `yaml:"enableWorkloadApprove,omitempty"`
}

type SecretBackendConfig struct {
	Name   string         `yaml:"name"`
	Driver string         `yaml:"driver"`
	Params map[string]any `yaml:"params"`
}

type ConfigLoaderConfig struct {
	Type                string              `yaml:"type"`
	SecretBackendParams SecretBackendParams `yaml:"secretBackendParams"`
}

type DomainRouteConfig struct {
	ExternalTLS   *kusciaconfig.TLSConfig `yaml:"externalTLS,omitempty"`
	DomainCsrData string                  `yaml:"-"`
}

func defaultMaster(rootDir string) KusciaConfig {
	conf := defaultKusciaConfig(rootDir)
	conf.Master = kusciaconfig.MasterConfig{
		APIServer: &kusciaconfig.APIServerConfig{
			KubeConfig: filepath.Join(conf.RootDir, "etc/kubeconfig"),
			Endpoint:   defaultEndpointForMaster,
		},
		KusciaAPI: &kusciaconfig.ServiceConfig{
			Endpoint: "http://127.0.0.1:8092",
		},
	}
	conf.DomainRoute = DomainRouteConfig{
		ExternalTLS: &kusciaconfig.TLSConfig{
			EnableTLS: true,
		},
	}
	return conf
}

func defaultLite(rootDir string) KusciaConfig {
	conf := defaultKusciaConfig(rootDir)
	conf.Agent = *config.DefaultAgentConfig()
	return conf
}

func defaultAutonomy(rootDir string) KusciaConfig {
	conf := defaultMaster(rootDir)
	conf.Agent = *config.DefaultAgentConfig()

	return conf
}

func defaultKusciaConfig(rootDir string) KusciaConfig {
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	if rootDir == "" {
		rootDir = defaultRootDir
	}
	return KusciaConfig{
		RootDir:            rootDir,
		CAKeyFile:          filepath.Join(rootDir, common.CertPrefix, "ca.key"),
		CACertFile:         filepath.Join(rootDir, common.CertPrefix, "ca.crt"),
		DomainKeyFile:      filepath.Join(rootDir, common.CertPrefix, "domain.key"),
		DomainCertFile:     filepath.Join(rootDir, common.CertPrefix, "domain.crt"),
		EnvoyIP:            hostIP,
		KusciaAPI:          kaconfig.NewDefaultKusciaAPIConfig(rootDir),
		MetricUpdatePeriod: defaultMetricUpdatePeriod,
		Logrotate:          LogrotateConfig{3, 128},
	}
}

func ReadConfig(configFile, runMode string) KusciaConfig {
	var conf KusciaConfig
	switch runMode {
	case common.RunModeMaster:
		masterConfig := LoadMasterConfig(configFile)
		conf = defaultMaster(defaultRootDir)
		masterConfig.OverwriteKusciaConfig(&conf)
	case common.RunModeLite:
		liteConfig := LoadLiteConfig(configFile)
		conf = defaultLite(defaultRootDir)
		liteConfig.OverwriteKusciaConfig(&conf)
	case common.RunModeAutonomy:
		autonomyConfig := LoadAutonomyConfig(configFile)
		conf = defaultAutonomy(defaultRootDir)
		autonomyConfig.OverwriteKusciaConfig(&conf)
	default:
		nlog.Fatalf("Not supported run mode: %s", runMode)
	}
	conf.RunMode = runMode
	if conf.DomainID == "" {
		nlog.Fatalf("Kuscia config domain should not be empty")
	}
	return conf
}
