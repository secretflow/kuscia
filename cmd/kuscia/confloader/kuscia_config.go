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
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	dmconfig "github.com/secretflow/kuscia/pkg/datamesh/config"
	kaconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
)

type LiteKusciaConfig struct {
	CommonConfig    `yaml:",inline"`
	LiteDeployToken string             `yaml:"liteDeployToken"`
	MasterEndpoint  string             `yaml:"masterEndpoint"`
	Runtime         string             `yaml:"runtime"`
	Runk            RunkConfig         `yaml:"runk"`
	Capacity        config.CapacityCfg `yaml:"capacity"`
	Image           ImageConfig        `yaml:"image"`
	AdvancedConfig  `yaml:",inline"`
}

type MasterKusciaConfig struct {
	CommonConfig      `yaml:",inline"`
	DatastoreEndpoint string `yaml:"datastoreEndpoint"`
	ClusterToken      string `yaml:"clusterToken,omitempty"`
	AdvancedConfig    `yaml:",inline"`
}

type AutomonyKusciaConfig struct {
	CommonConfig      `yaml:",inline"`
	Runtime           string             `yaml:"runtime"`
	Runk              RunkConfig         `yaml:"runk"`
	Capacity          config.CapacityCfg `yaml:"capacity"`
	Image             ImageConfig        `yaml:"image"`
	DatastoreEndpoint string             `yaml:"datastoreEndpoint"`
	AdvancedConfig    `yaml:",inline"`
}

type RunkConfig struct {
	Namespace      string   `yaml:"namespace"`
	DNSServers     []string `yaml:"dnsServers"`
	KubeconfigFile string   `yaml:"kubeconfigFile"`
}

func (runk RunkConfig) overwriteK8sProviderCfg(k8sCfg config.K8sProviderCfg) config.K8sProviderCfg {
	k8sCfg.Namespace = runk.Namespace
	k8sCfg.KubeconfigFile = runk.KubeconfigFile
	k8sCfg.DNS.Servers = runk.DNSServers
	return k8sCfg
}

type ImageConfig struct {
	PullPolicy      string          `yaml:"pullPolicy"`
	DefaultRegistry string          `yaml:"defaultRegistry"`
	Registries      []ImageRegistry `yaml:"registries"`
}

type ImageRegistry struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	UserName string `yaml:"username"`
	Password string `yaml:"password"`
}

type CommonConfig struct {
	Mode          string          `yaml:"mode"`
	DomainID      string          `yaml:"domainID"`
	DomainKeyData string          `yaml:"domainKeyData"`
	LogLevel      string          `yaml:"logLevel"`
	Protocol      common.Protocol `yaml:"protocol,omitempty"`
}

type LogrotateConfig struct {
	MaxFiles      int `yaml:"maxFiles"`
	MaxFileSizeMB int `yaml:"maxFileSizeMB"`
}

type AdvancedConfig struct {
	ConfLoaders           []ConfigLoaderConfig      `yaml:"confLoaders,omitempty"`
	SecretBackends        []SecretBackendConfig     `yaml:"secretBackends,omitempty"`
	KusciaAPI             *kaconfig.KusciaAPIConfig `yaml:"kusciaAPI,omitempty"`
	DataMesh              *dmconfig.DataMeshConfig  `yaml:"dataMesh,omitempty"`
	DomainRoute           DomainRouteConfig         `yaml:"domainRoute,omitempty"`
	Agent                 config.AgentConfig        `yaml:"agent,omitempty"`
	Debug                 bool                      `yaml:"debug,omitempty"`
	DebugPort             int                       `yaml:"debugPort,omitempty"`
	EnableWorkloadApprove bool                      `yaml:"enableWorkloadApprove,omitempty"`
	Logrotate             LogrotateConfig           `yaml:"logrotate,omitempty"`
}

func LoadCommonConfig(configFile string) *CommonConfig {
	conf := &CommonConfig{}
	loadConfig(configFile, conf)
	return conf
}

func LoadLiteConfig(configFile string) *LiteKusciaConfig {
	conf := &LiteKusciaConfig{}
	loadConfig(configFile, conf)
	return conf
}

func LoadMasterConfig(configFile string) *MasterKusciaConfig {
	conf := &MasterKusciaConfig{}
	loadConfig(configFile, conf)
	return conf
}

func LoadAutonomyConfig(configFile string) *AutomonyKusciaConfig {
	conf := &AutomonyKusciaConfig{}
	loadConfig(configFile, conf)
	return conf
}

func (lite *LiteKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.LogLevel = lite.LogLevel
	kusciaConfig.DomainID = lite.DomainID
	kusciaConfig.CAKeyData = lite.DomainKeyData
	kusciaConfig.DomainKeyData = lite.DomainKeyData
	kusciaConfig.ConfLoaders = lite.ConfLoaders
	kusciaConfig.SecretBackends = lite.SecretBackends
	if lite.KusciaAPI != nil {
		kusciaConfig.KusciaAPI = lite.KusciaAPI
	}
	kusciaConfig.Protocol = lite.Protocol
	kusciaConfig.DataMesh = lite.DataMesh
	kusciaConfig.Agent.AllowPrivileged = lite.Agent.AllowPrivileged
	kusciaConfig.Agent.Provider.Runtime = lite.Runtime
	kusciaConfig.Agent.Provider.K8s = lite.Runk.overwriteK8sProviderCfg(lite.Agent.Provider.K8s)
	kusciaConfig.Agent.Capacity = lite.Capacity

	for _, p := range lite.Agent.Plugins {
		for j, pp := range kusciaConfig.Agent.Plugins {
			if p.Name == pp.Name {
				kusciaConfig.Agent.Plugins[j] = p
				break
			}
		}
	}

	kusciaConfig.Master.Endpoint = lite.MasterEndpoint
	kusciaConfig.DomainRoute.DomainCsrData = generateCsrData(lite.DomainID, lite.DomainKeyData, lite.LiteDeployToken)
	kusciaConfig.Debug = lite.Debug
	kusciaConfig.DebugPort = lite.DebugPort

	overwriteKusciaConfigLogrotate(&kusciaConfig.Logrotate, &lite.AdvancedConfig.Logrotate)
}

func (master *MasterKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.DomainID = master.DomainID
	kusciaConfig.LogLevel = master.LogLevel
	kusciaConfig.CAKeyData = master.DomainKeyData
	kusciaConfig.DomainKeyData = master.DomainKeyData
	kusciaConfig.ConfLoaders = master.ConfLoaders
	kusciaConfig.SecretBackends = master.SecretBackends
	if master.KusciaAPI != nil {
		kusciaConfig.KusciaAPI = master.KusciaAPI
	}
	kusciaConfig.Protocol = master.Protocol
	if master.DomainRoute.ExternalTLS != nil {
		kusciaConfig.DomainRoute.ExternalTLS = master.DomainRoute.ExternalTLS
	}
	kusciaConfig.Master.DatastoreEndpoint = master.DatastoreEndpoint
	kusciaConfig.Master.ClusterToken = master.ClusterToken
	kusciaConfig.Debug = master.Debug
	kusciaConfig.DebugPort = master.DebugPort
	kusciaConfig.EnableWorkloadApprove = master.AdvancedConfig.EnableWorkloadApprove

	overwriteKusciaConfigLogrotate(&kusciaConfig.Logrotate, &master.AdvancedConfig.Logrotate)
}

func (autonomy *AutomonyKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.LogLevel = autonomy.LogLevel
	kusciaConfig.DomainID = autonomy.DomainID
	kusciaConfig.CAKeyData = autonomy.DomainKeyData
	kusciaConfig.DomainKeyData = autonomy.DomainKeyData
	kusciaConfig.Agent.AllowPrivileged = autonomy.Agent.AllowPrivileged
	kusciaConfig.Agent.Provider.Runtime = autonomy.Runtime
	kusciaConfig.Agent.Provider.K8s = autonomy.Runk.overwriteK8sProviderCfg(autonomy.Agent.Provider.K8s)
	kusciaConfig.Agent.Capacity = autonomy.Capacity

	for _, p := range autonomy.Agent.Plugins {
		for j, pp := range kusciaConfig.Agent.Plugins {
			if p.Name == pp.Name {
				kusciaConfig.Agent.Plugins[j] = p
				break
			}
		}
	}

	kusciaConfig.ConfLoaders = autonomy.ConfLoaders
	kusciaConfig.SecretBackends = autonomy.SecretBackends
	if autonomy.KusciaAPI != nil {
		kusciaConfig.KusciaAPI = autonomy.KusciaAPI
	}
	kusciaConfig.Protocol = autonomy.Protocol
	kusciaConfig.DataMesh = autonomy.DataMesh
	if autonomy.DomainRoute.ExternalTLS != nil {
		kusciaConfig.DomainRoute.ExternalTLS = autonomy.DomainRoute.ExternalTLS
	}
	kusciaConfig.Master.DatastoreEndpoint = autonomy.DatastoreEndpoint
	kusciaConfig.Debug = autonomy.Debug
	kusciaConfig.DebugPort = autonomy.DebugPort
	kusciaConfig.EnableWorkloadApprove = autonomy.AdvancedConfig.EnableWorkloadApprove

	overwriteKusciaConfigLogrotate(&kusciaConfig.Logrotate, &autonomy.AdvancedConfig.Logrotate)
}

func overwriteKusciaConfigLogrotate(kusciaConfig, overwriteLogrotate *LogrotateConfig) {
	if overwriteLogrotate != nil {
		if overwriteLogrotate.MaxFileSizeMB > 0 {
			kusciaConfig.MaxFileSizeMB = overwriteLogrotate.MaxFileSizeMB
		}
		if overwriteLogrotate.MaxFiles > 0 {
			kusciaConfig.MaxFiles = overwriteLogrotate.MaxFiles
		}
	}
}

func loadConfig(configFile string, conf interface{}) {
	content, err := os.ReadFile(configFile)
	if err != nil {
		nlog.Fatal(err)
	}
	if err = yaml.Unmarshal(content, conf); err != nil {
		nlog.Fatal(err)
	}
}

func generateCsrData(domainID, domainKeyData, deployToken string) string {
	domainKeyDataDecoded, err := base64.StdEncoding.DecodeString(domainKeyData)
	if err != nil {
		nlog.Fatalf("Load domain key file error: %v", err.Error())
	}

	key, err := tlsutils.ParseKey(domainKeyDataDecoded, "")
	if err != nil {
		nlog.Fatalf("Decode domain key data error: %v", err.Error())
	}

	extensionIDs := strings.Split(common.DomainCsrExtensionID, ".")
	var asn1Id asn1.ObjectIdentifier
	for _, str := range extensionIDs {
		id, err := strconv.Atoi(str)
		if err != nil {
			nlog.Fatalf("Parse extension ID error: %v", err.Error())
		}
		asn1Id = append(asn1Id, id)
	}

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: domainID,
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
		ExtraExtensions: []pkix.Extension{
			{
				Id:    asn1Id,
				Value: []byte(deployToken),
			},
		},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		nlog.Fatalf("Create x509 certificate request error: %v", err)
	}

	var reader bytes.Buffer
	if err := pem.Encode(&reader, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}); err != nil {
		nlog.Fatalf("Encode certificate request error: %v", err.Error())
	}
	return reader.String()
}
