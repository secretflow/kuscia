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

package confloader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestAgentLogConfigOverwrite(t *testing.T) {
	autonomy := &AutonomyKusciaConfig{
		AdvancedConfig: AdvancedConfig{
			Agent: config.AgentConfig{
				Provider: config.ProviderCfg{
					CRI: config.CRIProviderCfg{
						ContainerLogMaxSize:  "128Mi",
						ContainerLogMaxFiles: 3,
					},
				},
			},
		},
	}

	kusciaConfig := &AutonomyKusciaConfig{
		AdvancedConfig: AdvancedConfig{
			Logrotate: LogrotateConfig{
				MaxFiles:      7,
				MaxFileSizeMB: 256,
			},
			Agent: config.AgentConfig{
				Provider: config.ProviderCfg{
					CRI: config.CRIProviderCfg{
						ContainerLogMaxSize:  "512Mi",
						ContainerLogMaxFiles: 5,
					},
				},
			},
		},
	}

	overwriteKusciaConfigAgentLogrotate(&kusciaConfig.Agent.Provider.CRI, &autonomy.Agent.Provider.CRI, &kusciaConfig.Logrotate)

	resultCfg := config.CRIProviderCfg{
		ContainerLogMaxSize:  "128Mi",
		ContainerLogMaxFiles: 3,
	}
	assert.True(t, reflect.DeepEqual(kusciaConfig.Agent.Provider.CRI, resultCfg))

	// invalid agent cri, use kuscia log rotate
	autonomy.Agent.Provider.CRI.ContainerLogMaxSize = "128m"
	autonomy.Agent.Provider.CRI.ContainerLogMaxFiles = 0
	overwriteKusciaConfigAgentLogrotate(&kusciaConfig.Agent.Provider.CRI, &autonomy.Agent.Provider.CRI, &kusciaConfig.Logrotate)
	resultCfg.ContainerLogMaxFiles = 7
	resultCfg.ContainerLogMaxSize = "256Mi"
	assert.True(t, reflect.DeepEqual(kusciaConfig.Agent.Provider.CRI, resultCfg))

	// invalid kuscia log rotate, use default (unchanged)
	kusciaConfig.Logrotate.MaxFiles = 0
	kusciaConfig.Logrotate.MaxFileSizeMB = 0
	overwriteKusciaConfigAgentLogrotate(&kusciaConfig.Agent.Provider.CRI, &autonomy.Agent.Provider.CRI, &kusciaConfig.Logrotate)
	assert.True(t, reflect.DeepEqual(kusciaConfig.Agent.Provider.CRI, resultCfg))
}

func TestAutonomyOverwriteKusciaConfig(t *testing.T) {
	autonomy := common.RunModeAutonomy
	domainKeyData, err := tls.GenerateKeyData()
	assert.NoError(t, err)
	kusciaConfig := &AutonomyKusciaConfig{
		CommonConfig: CommonConfig{
			Mode:          autonomy,
			DomainID:      "alice",
			DomainKeyData: domainKeyData,
			Protocol:      common.TLS,
		},
		Runtime: config.K8sRuntime,
		Runk: RunkConfig{
			LogMaxSize:   "128Mi",
			LogMaxFiles:  3,
			LogDirectory: "/test/dir",
		},
		AdvancedConfig: AdvancedConfig{
			Logrotate: LogrotateConfig{
				MaxFiles:      7,
				MaxFileSizeMB: 256,
			},
			Agent: config.AgentConfig{
				KusciaAPIProtocol: common.MTLS,
				Provider: config.ProviderCfg{
					CRI: config.CRIProviderCfg{
						ContainerLogMaxSize:  "512Mi",
						ContainerLogMaxFiles: 5,
					},
				},
			},
		},
		ReservedResources: config.ReservedResourcesCfg{
			CPU:    "100m",
			Memory: "100Mi",
		},
	}

	data, err := yaml.Marshal(kusciaConfig)
	assert.NoError(t, err)
	dir := t.TempDir()

	filename := filepath.Join(dir, "kuscia.yaml")
	assert.NoError(t, os.WriteFile(filename, data, 600))

	autonomyConfig := LoadAutonomyConfig(filename)
	// conf := defaultAutonomy(common.DefaultKusciaHomePath)
	conf := KusciaConfig{
		RunMode: common.RunModeAutonomy,
		Agent: config.AgentConfig{
			KusciaAPIProtocol: common.MTLS,
		},
	}
	// use autonomyConfig(from file) to overwrite conf
	autonomyConfig.OverwriteKusciaConfig(&conf)

	resultCfg := KusciaConfig{
		Agent:         kusciaConfig.Agent,
		RunMode:       kusciaConfig.Mode,
		DomainID:      kusciaConfig.DomainID,
		DomainKeyData: kusciaConfig.DomainKeyData,
		CAKeyData:     kusciaConfig.DomainKeyData,
		Protocol:      kusciaConfig.Protocol,
		Logrotate:     kusciaConfig.Logrotate,
	}
	resultCfg.Agent.Provider.Runtime = config.K8sRuntime
	resultCfg.Agent.StdoutPath = "/test/dir"
	resultCfg.Agent.Provider.K8s.LogDirectory = "/test/dir"
	resultCfg.Agent.Provider.K8s.LogMaxFiles = 3
	resultCfg.Agent.Provider.K8s.LogMaxSize = "128Mi"
	resultCfg.Agent.ReservedResources = config.ReservedResourcesCfg{
		CPU:    "100m",
		Memory: "100Mi",
	}

	data1, err := yaml.Marshal(resultCfg)
	assert.NoError(t, err)
	data2, err := yaml.Marshal(conf)
	assert.NoError(t, err)
	assert.Equal(t, data1, data2)

	// other branch
	autonomyConfig.Logrotate.MaxFileSizeMB = 0
	autonomyConfig.Logrotate.MaxFiles = 0
	autonomyConfig.Runk = RunkConfig{}
	conf = KusciaConfig{
		RunMode: common.RunModeAutonomy,
		Agent: config.AgentConfig{
			KusciaAPIProtocol: common.MTLS,
		},
		Logrotate: LogrotateConfig{
			MaxFiles:      9,
			MaxFileSizeMB: 400,
		},
	}
	// use autonomyConfig(from file) to overwrite conf
	autonomyConfig.OverwriteKusciaConfig(&conf)
	resultCfg.Agent.Provider.K8s.LogMaxFiles = 9
	resultCfg.Agent.Provider.K8s.LogMaxSize = "400Mi"
	resultCfg.Logrotate.MaxFileSizeMB = 400
	resultCfg.Logrotate.MaxFiles = 9
	resultCfg.Agent.StdoutPath = ""
	resultCfg.Agent.Provider.K8s.LogDirectory = ""
	data1, err = yaml.Marshal(resultCfg)
	assert.NoError(t, err)
	data2, err = yaml.Marshal(conf)
	assert.NoError(t, err)

	filename = filepath.Join(dir, "kuscia1.yaml")
	assert.NoError(t, os.WriteFile(filename, data1, 600))
	filename = filepath.Join(dir, "kuscia2.yaml")
	assert.NoError(t, os.WriteFile(filename, data2, 600))

	assert.Equal(t, data1, data2)
}

func TestLiteOverwriteKusciaConfig(t *testing.T) {
	mode := common.RunModeLite
	domainKeyData, err := tls.GenerateKeyData()
	assert.NoError(t, err)
	kusciaConfig := &LiteKusciaConfig{
		CommonConfig: CommonConfig{
			Mode:          mode,
			DomainID:      "alice",
			DomainKeyData: domainKeyData,
			Protocol:      common.TLS,
		},
		Runk: RunkConfig{
			LogMaxSize:  "128Mi",
			LogMaxFiles: 3,
		},
		AdvancedConfig: AdvancedConfig{
			Logrotate: LogrotateConfig{
				MaxFiles:      7,
				MaxFileSizeMB: 256,
			},
			Agent: config.AgentConfig{
				KusciaAPIProtocol: common.MTLS,
				Provider: config.ProviderCfg{
					CRI: config.CRIProviderCfg{
						ContainerLogMaxSize:  "512Mi",
						ContainerLogMaxFiles: 5,
					},
				},
			},
		},
		ReservedResources: config.ReservedResourcesCfg{
			CPU:    "100m",
			Memory: "100Mi",
		},
	}

	data, err := yaml.Marshal(kusciaConfig)
	assert.NoError(t, err)
	dir := t.TempDir()

	filename := filepath.Join(dir, "kuscia.yaml")
	assert.NoError(t, os.WriteFile(filename, data, 600))

	liteConfig := LoadLiteConfig(filename)
	// conf := defaultAutonomy(common.DefaultKusciaHomePath)
	conf := KusciaConfig{
		RunMode: mode,
		Agent: config.AgentConfig{
			KusciaAPIProtocol: common.MTLS,
		},
	}
	// use autonomyConfig(from file) to overwrite conf
	liteConfig.OverwriteKusciaConfig(&conf)

	resultCfg := KusciaConfig{
		Agent:         kusciaConfig.Agent,
		RunMode:       kusciaConfig.Mode,
		DomainID:      kusciaConfig.DomainID,
		DomainKeyData: kusciaConfig.DomainKeyData,
		CAKeyData:     kusciaConfig.DomainKeyData,
		Protocol:      kusciaConfig.Protocol,
		Logrotate:     kusciaConfig.Logrotate,
		DomainRoute: DomainRouteConfig{
			DomainCsrData: GenerateCsrData(kusciaConfig.DomainID, kusciaConfig.DomainKeyData, kusciaConfig.LiteDeployToken),
		},
	}
	resultCfg.Agent.Provider.K8s.LogMaxFiles = 3
	resultCfg.Agent.Provider.K8s.LogMaxSize = "128Mi"
	resultCfg.Agent.ReservedResources = config.ReservedResourcesCfg{
		CPU:    "100m",
		Memory: "100Mi",
	}

	data1, err := yaml.Marshal(resultCfg)
	assert.NoError(t, err)
	data2, err := yaml.Marshal(conf)
	assert.NoError(t, err)
	assert.Equal(t, data1, data2)

}

func TestMasterOverwriteKusciaConfig(t *testing.T) {
	mode := common.RunModeMaster
	domainKeyData, err := tls.GenerateKeyData()
	assert.NoError(t, err)
	kusciaConfig := &LiteKusciaConfig{
		CommonConfig: CommonConfig{
			Mode:          mode,
			DomainID:      "alice",
			DomainKeyData: domainKeyData,
			Protocol:      common.TLS,
		},
		AdvancedConfig: AdvancedConfig{
			Logrotate: LogrotateConfig{
				MaxFiles:      7,
				MaxFileSizeMB: 256,
			},
		},
	}

	data, err := yaml.Marshal(kusciaConfig)
	assert.NoError(t, err)
	dir := t.TempDir()

	filename := filepath.Join(dir, "kuscia.yaml")
	assert.NoError(t, os.WriteFile(filename, data, 600))

	liteConfig := LoadMasterConfig(filename)
	// conf := defaultAutonomy(common.DefaultKusciaHomePath)
	conf := KusciaConfig{
		RunMode: mode,
	}
	// use autonomyConfig(from file) to overwrite conf
	liteConfig.OverwriteKusciaConfig(&conf)

	resultCfg := KusciaConfig{
		RunMode:       kusciaConfig.Mode,
		DomainID:      kusciaConfig.DomainID,
		DomainKeyData: kusciaConfig.DomainKeyData,
		CAKeyData:     kusciaConfig.DomainKeyData,
		Protocol:      kusciaConfig.Protocol,
		Logrotate:     kusciaConfig.Logrotate,
	}

	data1, err := yaml.Marshal(resultCfg)
	assert.NoError(t, err)
	data2, err := yaml.Marshal(conf)
	assert.NoError(t, err)

	filename = filepath.Join(dir, "kuscia1.yaml")
	assert.NoError(t, os.WriteFile(filename, data1, 600))
	filename = filepath.Join(dir, "kuscia2.yaml")
	assert.NoError(t, os.WriteFile(filename, data2, 600))

	assert.Equal(t, data1, data2)

}
