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
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

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
	defaultRootDir           = "/home/kuscia/"
	defaultEndpointForMaster = "https://127.0.0.1:6443"
	certPrefix               = "etc/certs/"
	tmpDirPrefix             = "var/tmp/"
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

	Agent          config.AgentConfig        `yaml:"agent,omitempty"`
	Master         kusciaconfig.MasterConfig `yaml:"master,omitempty"`
	ConfManager    cmconf.ConfManagerConfig  `yaml:"confManager,omitempty"`
	ExternalTLS    *kusciaconfig.TLSConfig   `yaml:"externalTLS,omitempty"`
	KusciaAPI      *kaconfig.KusciaAPIConfig `yaml:"kusciaAPI,omitempty"`
	SecretBackends []SecretBackendConfig     `yaml:"secretBackends,omitempty"`
	ConfLoaders    []ConfigLoaderConfig      `yaml:"confLoaders,omitempty"`
	DataMesh       *dmconfig.DataMeshConfig  `yaml:"dataMesh,omitempty"`

	EnvoyIP           string             `yaml:"-"`
	CoreDNSBackUpConf string             `yaml:"-"`
	RunMode           common.RunModeType `yaml:"-"`
}

type SecretBackendConfig struct {
	Name                       string `yaml:"name"`
	cmconf.SecretBackendConfig `yaml:",inline"`
}

type ConfigLoaderConfig struct {
	Type                string              `yaml:"type"`
	SecretBackendParams SecretBackendParams `yaml:"secretBackendParams"`
}

func defaultMaster() KusciaConfig {
	conf := defaultKusciaConfig()
	conf.Master = kusciaconfig.MasterConfig{
		APIServer: &kusciaconfig.APIServerConfig{
			KubeConfig: filepath.Join(conf.RootDir, "etc/kubeconfig"),
			Endpoint:   defaultEndpointForMaster,
		},
		KusciaAPI: &kusciaconfig.ServiceConfig{
			Endpoint: "http://127.0.0.1:8092",
		},
	}
	return conf
}

func defaultLite() KusciaConfig {
	conf := defaultKusciaConfig()
	conf.Agent = *config.DefaultAgentConfig()
	// TODO: gateway: no conf.Master.APIServer == nil is master

	return conf
}

func defaultAutonomy() KusciaConfig {
	conf := defaultMaster()
	conf.Agent = *config.DefaultAgentConfig()

	return conf
}

func defaultKusciaConfig() KusciaConfig {
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	return KusciaConfig{
		RootDir:        defaultRootDir,
		CAKeyFile:      filepath.Join(defaultRootDir, certPrefix, "ca.key"),
		CACertFile:     filepath.Join(defaultRootDir, certPrefix, "ca.crt"),
		DomainKeyFile:  filepath.Join(defaultRootDir, certPrefix, "domain.key"),
		DomainCertFile: filepath.Join(defaultRootDir, tmpDirPrefix, "domain.crt"),
		EnvoyIP:        hostIP,
	}
}

func ReadConfig(configFile string, flagDomainID string, runMode string) KusciaConfig {
	var conf KusciaConfig
	switch runMode {
	case common.RunModeMaster:
		conf = defaultMaster()
	case common.RunModeLite:
		conf = defaultLite()
	case common.RunModeAutonomy:
		conf = defaultAutonomy()
	default:
		nlog.Fatalf("Not supported run mode: %s", runMode)
	}
	conf.RunMode = runMode
	// overwrite config
	if configFile != "" {
		content, err := os.ReadFile(configFile)
		if err != nil {
			nlog.Fatal(err)
		}
		err = yaml.Unmarshal(content, &conf)
		if err != nil {
			nlog.Fatal(err)
		}
	}
	// overwrite by domain flag
	if flagDomainID != "" {
		conf.DomainID = flagDomainID
	}
	if conf.DomainID == "" {
		nlog.Fatalf("Kuscia config domain should not be empty")
	}
	return conf
}
