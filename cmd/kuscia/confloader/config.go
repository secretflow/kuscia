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
	"fmt"
	"path/filepath"
	"strings"

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

	Image ImageConfig `yaml:"image"`

	Agent                 config.AgentConfig        `yaml:"agent,omitempty"`
	Master                kusciaconfig.MasterConfig `yaml:"master,omitempty"`
	ConfManager           *cmconf.ConfManagerConfig `yaml:"confManager,omitempty"`
	KusciaAPI             *kaconfig.KusciaAPIConfig `yaml:"kusciaAPI,omitempty"`
	DataMesh              *dmconfig.DataMeshConfig  `yaml:"dataMesh,omitempty"`
	DomainRoute           DomainRouteConfig         `yaml:"domainRoute,omitempty"`
	Protocol              common.Protocol           `yaml:"protocol"`
	EnvoyIP               string                    `yaml:"-"`
	CoreDNSBackUpConf     string                    `yaml:"-"`
	RunMode               common.RunModeType        `yaml:"-"`
	EnableWorkloadApprove bool                      `yaml:"enableWorkloadApprove,omitempty"`

	// Garbage Collection configuration
	GarbageCollection GarbageCollectionConfig `yaml:"garbageCollection,omitempty"`
}

// GarbageCollectionConfig defines configuration for garbage collection controllers
type GarbageCollectionConfig struct {
	// KusciaDomainDataGC configuration for kuscia domain data garbage collection
	KusciaDomainDataGC KusciaDomainDataGCConfig `yaml:"kusciaDomainDataGC,omitempty"`
	// KusciaJobGC configuration for kuscia job garbage collection
	KusciaJobGC KusciaJobGCConfig `yaml:"kusciaJobGC,omitempty"`
}

type KusciaDomainDataGCConfig struct {
	// Enable controls whether kuscia domain data garbage collection is enabled
	Enable *bool `yaml:"enable,omitempty"`
	// DurationHours specifies how many hours to retain domain data before garbage collection. Default is 720 hours (30 days).
	DurationHours int `yaml:"durationHours,omitempty"`
}

// KusciaJobGCConfig defines configuration specific to kuscia job garbage collection
type KusciaJobGCConfig struct {
	// DurationHours specifies how many hours to retain completed kuscia jobs before garbage collection. Default is 720 hours (30 days).
	DurationHours int `yaml:"durationHours,omitempty"`
}
type CMConfig struct {
	Params map[string]any `yaml:"params,omitempty"`
	Driver string         `yaml:"driver,omitempty"`
}

type DomainRouteConfig struct {
	ExternalTLS   *kusciaconfig.TLSConfig `yaml:"externalTLS,omitempty"`
	DomainCsrData string                  `yaml:"-"`
}

func defaultMaster(rootDir string) KusciaConfig {
	conf := DefaultKusciaConfig(rootDir)
	conf.Master = kusciaconfig.MasterConfig{
		APIServer: &kusciaconfig.APIServerConfig{
			KubeConfig: filepath.Join(conf.RootDir, "etc/kubeconfig"),
			Endpoint:   defaultEndpointForMaster,
		},
		KusciaAPI: &kusciaconfig.ServiceConfig{
			Endpoint: "http://127.0.0.1:8092",
		},
		Reporter: &kusciaconfig.ServiceConfig{
			Endpoint: "http://127.0.0.1:8050",
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
	conf := DefaultKusciaConfig(rootDir)
	conf.Agent = *config.DefaultAgentConfig(rootDir)
	return conf
}

func defaultAutonomy(rootDir string) KusciaConfig {
	conf := defaultMaster(rootDir)
	conf.Agent = *config.DefaultAgentConfig(rootDir)

	return conf
}

func DefaultKusciaConfig(rootDir string) KusciaConfig {
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	if rootDir == "" {
		rootDir = common.DefaultKusciaHomePath()
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
		Logrotate: LogrotateConfig{
			MaxFiles:      config.DefaultLogRotateMaxFiles,
			MaxFileSizeMB: config.DefaultLogRotateMaxSize,
			MaxAgeDays:    config.DefaultLogRotateMaxAgeDays,
		},
	}
}

func ReadConfig(configFile string) (KusciaConfig, error) {

	var conf KusciaConfig

	commonConfig, err := LoadCommonConfig(configFile)
	if err != nil {
		return conf, err
	}

	//If the user configured RunMode{Autonomy,Lite,Master, LITE}
	// in uppercase in kuscia.yml, convert it to lowercase
	currentRunMode := strings.ToLower(commonConfig.Mode)

	switch currentRunMode {
	case common.RunModeMaster:
		masterConfig, err := LoadMasterConfig(configFile)
		if err != nil {
			return conf, err
		}
		conf = defaultMaster(common.DefaultKusciaHomePath())
		masterConfig.OverwriteKusciaConfig(&conf)
	case common.RunModeLite:
		liteConfig, err := LoadLiteConfig(configFile)
		if err != nil {
			return conf, err
		}
		conf = defaultLite(common.DefaultKusciaHomePath())
		liteConfig.OverwriteKusciaConfig(&conf)
	case common.RunModeAutonomy:
		autonomyConfig, err := LoadAutonomyConfig(configFile)
		if err != nil {
			return conf, err
		}
		conf = defaultAutonomy(common.DefaultKusciaHomePath())
		autonomyConfig.OverwriteKusciaConfig(&conf)
	default:
		return conf, fmt.Errorf("not support run mode: %s", currentRunMode)
	}
	if conf.DomainID == "" {
		return conf, fmt.Errorf("kuscia config domain should not be empty")
	}

	// Setting current run mode.
	// The configuration is strongly bound to the runtime#NewModuleRuntimeConfigs.
	conf.RunMode = currentRunMode

	return conf, nil
}
