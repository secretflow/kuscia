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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	cmconfig "github.com/secretflow/kuscia/pkg/confmanager/config"
	dmconfig "github.com/secretflow/kuscia/pkg/datamesh/config"
	kaconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
)

type LiteKusciaConfig struct {
	CommonConfig      `yaml:",inline"`
	LiteDeployToken   string                      `yaml:"liteDeployToken"`
	MasterEndpoint    string                      `yaml:"masterEndpoint"`
	Runtime           string                      `yaml:"runtime"`
	Runk              RunkConfig                  `yaml:"runk"`
	Capacity          config.CapacityCfg          `yaml:"capacity"`
	ReservedResources config.ReservedResourcesCfg `yaml:"reservedResources"`
	Image             ImageConfig                 `yaml:"image"`
	AdvancedConfig    `yaml:",inline"`
}

type MasterKusciaConfig struct {
	CommonConfig      `yaml:",inline"`
	DatastoreEndpoint string `yaml:"datastoreEndpoint"`
	ClusterToken      string `yaml:"clusterToken,omitempty"`
	AdvancedConfig    `yaml:",inline"`
}

type AutonomyKusciaConfig struct {
	CommonConfig      `yaml:",inline"`
	Runtime           string                      `yaml:"runtime"`
	Runk              RunkConfig                  `yaml:"runk"`
	Capacity          config.CapacityCfg          `yaml:"capacity"`
	ReservedResources config.ReservedResourcesCfg `yaml:"reservedResources"`
	Image             ImageConfig                 `yaml:"image"`
	DatastoreEndpoint string                      `yaml:"datastoreEndpoint"`
	AdvancedConfig    `yaml:",inline"`
}

type RunkConfig struct {
	Namespace      string   `yaml:"namespace"`
	DNSServers     []string `yaml:"dnsServers"`
	KubeconfigFile string   `yaml:"kubeconfigFile"`
	EnableLogging  bool     `yaml:"enableLogging"`
	LogDirectory   string   `yaml:"logDirectory"`
	LogMaxFiles    int      `yaml:"logMaxFiles"`
	LogMaxSize     string   `yaml:"logMaxSize"`
}

func (runk RunkConfig) overwriteK8sProviderCfg(k8sCfg config.K8sProviderCfg) config.K8sProviderCfg {
	k8sCfg.Namespace = runk.Namespace
	k8sCfg.KubeconfigFile = runk.KubeconfigFile
	k8sCfg.DNS.Servers = runk.DNSServers
	k8sCfg.EnableLogging = runk.EnableLogging
	k8sCfg.LogDirectory = runk.LogDirectory
	k8sCfg.LogMaxFiles = runk.LogMaxFiles
	k8sCfg.LogMaxSize = runk.LogMaxSize
	return k8sCfg
}

type ImageConfig struct {
	PullPolicy      string          `yaml:"pullPolicy"`
	DefaultRegistry string          `yaml:"defaultRegistry"`
	Registries      []ImageRegistry `yaml:"registries"`
	HTTPProxy       string          `yaml:"httpProxy"`
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
	MaxAgeDays    int `yaml:"maxAgeDays"`
}

type AdvancedConfig struct {
	KusciaAPI             *kaconfig.KusciaAPIConfig   `yaml:"kusciaAPI,omitempty"`
	ConfManager           *cmconfig.ConfManagerConfig `yaml:"confManager,omitempty"`
	DataMesh              *dmconfig.DataMeshConfig    `yaml:"dataMesh,omitempty"`
	DomainRoute           DomainRouteConfig           `yaml:"domainRoute,omitempty"`
	Agent                 config.AgentConfig          `yaml:"agent,omitempty"`
	Debug                 bool                        `yaml:"debug,omitempty"`
	DebugPort             int                         `yaml:"debugPort,omitempty"`
	EnableWorkloadApprove bool                        `yaml:"enableWorkloadApprove,omitempty"`
	Logrotate             LogrotateConfig             `yaml:"logrotate,omitempty"`
}

func LoadCommonConfig(configFile string) (*CommonConfig, error) {

	conf := &CommonConfig{}
	err := loadConfig(configFile, conf)
	return conf, err
}

func LoadLiteConfig(configFile string) (*LiteKusciaConfig, error) {

	conf := &LiteKusciaConfig{}
	err := loadConfig(configFile, conf)
	return conf, err
}

func LoadMasterConfig(configFile string) (*MasterKusciaConfig, error) {

	conf := &MasterKusciaConfig{}
	err := loadConfig(configFile, conf)
	return conf, err
}

func LoadAutonomyConfig(configFile string) (*AutonomyKusciaConfig, error) {

	conf := &AutonomyKusciaConfig{}
	err := loadConfig(configFile, conf)
	return conf, err
}

func (lite *LiteKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.LogLevel = lite.LogLevel
	kusciaConfig.DomainID = lite.DomainID
	kusciaConfig.CAKeyData = lite.DomainKeyData
	kusciaConfig.DomainKeyData = lite.DomainKeyData
	if lite.KusciaAPI != nil {
		kusciaConfig.KusciaAPI = lite.KusciaAPI
	}
	kusciaConfig.Protocol = lite.Protocol
	kusciaConfig.ConfManager = lite.ConfManager
	kusciaConfig.DataMesh = lite.DataMesh
	kusciaConfig.Agent.AllowPrivileged = lite.Agent.AllowPrivileged
	kusciaConfig.Agent.Provider.Runtime = lite.Runtime
	kusciaConfig.Agent.Provider.K8s = lite.Runk.overwriteK8sProviderCfg(lite.Agent.Provider.K8s)
	// overwrite runk log rotate config
	if kusciaConfig.Agent.Provider.K8s.LogMaxFiles <= 1 {
		if lite.AdvancedConfig.Logrotate.MaxFiles > 1 {
			kusciaConfig.Agent.Provider.K8s.LogMaxFiles = lite.AdvancedConfig.Logrotate.MaxFiles
		} else {
			kusciaConfig.Agent.Provider.K8s.LogMaxFiles = kusciaConfig.Logrotate.MaxFiles
		}
	}
	if _, parseErr := parseMaxSize(kusciaConfig.Agent.Provider.K8s.LogMaxSize); parseErr != nil {
		if lite.AdvancedConfig.Logrotate.MaxFileSizeMB > 0 {
			kusciaConfig.Agent.Provider.K8s.LogMaxSize = fmt.Sprintf("%dMi", lite.AdvancedConfig.Logrotate.MaxFileSizeMB)
		} else {
			kusciaConfig.Agent.Provider.K8s.LogMaxSize = fmt.Sprintf("%dMi", kusciaConfig.Logrotate.MaxFileSizeMB)
		}
	}
	kusciaConfig.Agent.Capacity = lite.Capacity
	if kusciaConfig.Agent.Provider.Runtime == config.K8sRuntime && kusciaConfig.Agent.Provider.K8s.LogDirectory != "" {
		kusciaConfig.Agent.StdoutPath = kusciaConfig.Agent.Provider.K8s.LogDirectory
	}

	if lite.ReservedResources.CPU != "" {
		kusciaConfig.Agent.ReservedResources.CPU = lite.ReservedResources.CPU
	}
	if lite.ReservedResources.Memory != "" {
		kusciaConfig.Agent.ReservedResources.Memory = lite.ReservedResources.Memory
	}
	if lite.ReservedResources.Bandwidth != "" {
		kusciaConfig.Agent.ReservedResources.Bandwidth = lite.ReservedResources.Bandwidth
	}

	for _, p := range lite.Agent.Plugins {
		for j, pp := range kusciaConfig.Agent.Plugins {
			if p.Name == pp.Name {
				kusciaConfig.Agent.Plugins[j] = p
				break
			}
		}
	}

	kusciaConfig.Master.Endpoint = lite.MasterEndpoint
	kusciaConfig.DomainRoute.DomainCsrData = GenerateCsrData(lite.DomainID, lite.DomainKeyData, lite.LiteDeployToken)
	kusciaConfig.Debug = lite.Debug
	kusciaConfig.DebugPort = lite.DebugPort
	kusciaConfig.Image = lite.Image
	kusciaConfig.Image.HTTPProxy = lite.Image.HTTPProxy

	overwriteKusciaConfigLogrotate(&kusciaConfig.Logrotate, &lite.AdvancedConfig.Logrotate)
	kusciaConfig.Agent.StdoutGCDuration = time.Duration(kusciaConfig.Logrotate.MaxAgeDays) * 24 * time.Hour
	overwriteKusciaConfigAgentLogrotate(&kusciaConfig.Agent.Provider.CRI, &lite.Agent.Provider.CRI, &kusciaConfig.Logrotate)
}

func (master *MasterKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.DomainID = master.DomainID
	kusciaConfig.LogLevel = master.LogLevel
	kusciaConfig.CAKeyData = master.DomainKeyData
	kusciaConfig.DomainKeyData = master.DomainKeyData
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

func (autonomy *AutonomyKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.LogLevel = autonomy.LogLevel
	kusciaConfig.DomainID = autonomy.DomainID
	kusciaConfig.CAKeyData = autonomy.DomainKeyData
	kusciaConfig.DomainKeyData = autonomy.DomainKeyData
	kusciaConfig.Agent.AllowPrivileged = autonomy.Agent.AllowPrivileged
	kusciaConfig.Agent.Provider.Runtime = autonomy.Runtime
	kusciaConfig.Agent.Provider.K8s = autonomy.Runk.overwriteK8sProviderCfg(autonomy.Agent.Provider.K8s)
	// overwrite runk log rotate config
	// maxFile must > 1, maxFileSizeMB must can be parsed
	if kusciaConfig.Agent.Provider.K8s.LogMaxFiles <= 1 {
		if autonomy.AdvancedConfig.Logrotate.MaxFiles > 1 {
			kusciaConfig.Agent.Provider.K8s.LogMaxFiles = autonomy.AdvancedConfig.Logrotate.MaxFiles
		} else {
			// runk log rotate config has NO DEFAULT INIT value, use kusciaConfig's logrotate config as final backup value
			kusciaConfig.Agent.Provider.K8s.LogMaxFiles = kusciaConfig.Logrotate.MaxFiles
		}
	}
	// if logMaxSize is invalid, consider to use inherited value
	if _, parseErr := parseMaxSize(kusciaConfig.Agent.Provider.K8s.LogMaxSize); parseErr != nil {
		if autonomy.AdvancedConfig.Logrotate.MaxFileSizeMB > 0 {
			kusciaConfig.Agent.Provider.K8s.LogMaxSize = fmt.Sprintf("%dMi", autonomy.AdvancedConfig.Logrotate.MaxFileSizeMB)
		} else {
			kusciaConfig.Agent.Provider.K8s.LogMaxSize = fmt.Sprintf("%dMi", kusciaConfig.Logrotate.MaxFileSizeMB)
		}
	}
	kusciaConfig.Agent.Capacity = autonomy.Capacity
	if kusciaConfig.Agent.Provider.Runtime == config.K8sRuntime && kusciaConfig.Agent.Provider.K8s.LogDirectory != "" {
		kusciaConfig.Agent.StdoutPath = kusciaConfig.Agent.Provider.K8s.LogDirectory
	}
	if autonomy.ReservedResources.CPU != "" {
		kusciaConfig.Agent.ReservedResources.CPU = autonomy.ReservedResources.CPU
	}
	if autonomy.ReservedResources.Memory != "" {
		kusciaConfig.Agent.ReservedResources.Memory = autonomy.ReservedResources.Memory
	}
	if autonomy.ReservedResources.Bandwidth != "" {
		kusciaConfig.Agent.ReservedResources.Bandwidth = autonomy.ReservedResources.Bandwidth
	}

	for _, p := range autonomy.Agent.Plugins {
		for j, pp := range kusciaConfig.Agent.Plugins {
			if p.Name == pp.Name {
				kusciaConfig.Agent.Plugins[j] = p
				break
			}
		}
	}

	if autonomy.KusciaAPI != nil {
		kusciaConfig.KusciaAPI = autonomy.KusciaAPI
	}
	kusciaConfig.Protocol = autonomy.Protocol
	kusciaConfig.ConfManager = autonomy.ConfManager
	kusciaConfig.DataMesh = autonomy.DataMesh
	if autonomy.DomainRoute.ExternalTLS != nil {
		kusciaConfig.DomainRoute.ExternalTLS = autonomy.DomainRoute.ExternalTLS
	}
	kusciaConfig.Master.DatastoreEndpoint = autonomy.DatastoreEndpoint
	kusciaConfig.Debug = autonomy.Debug
	kusciaConfig.DebugPort = autonomy.DebugPort
	kusciaConfig.EnableWorkloadApprove = autonomy.AdvancedConfig.EnableWorkloadApprove
	kusciaConfig.Image = autonomy.Image
	kusciaConfig.Image.HTTPProxy = autonomy.Image.HTTPProxy

	overwriteKusciaConfigLogrotate(&kusciaConfig.Logrotate, &autonomy.AdvancedConfig.Logrotate)
	kusciaConfig.Agent.StdoutGCDuration = time.Duration(kusciaConfig.Logrotate.MaxAgeDays) * 24 * time.Hour
	overwriteKusciaConfigAgentLogrotate(&kusciaConfig.Agent.Provider.CRI, &autonomy.Agent.Provider.CRI, &kusciaConfig.Logrotate)
}

// try to overwrite app's (secretflow) default logrotate config with kuscia yaml logrotate config or agent cri logrotate config
func overwriteKusciaConfigAgentLogrotate(kusciaAgentConfig, overwriteAgentLogrotate *config.CRIProviderCfg, overwriteLogrotate *LogrotateConfig) {
	// kusciaAgentConfig has a default initial value, so we only need to try overwrite, priority use cri logrotate config
	// maxFile must > 1, maxFileSizeMB must can be parsed
	if overwriteAgentLogrotate != nil && overwriteAgentLogrotate.ContainerLogMaxFiles > 1 {
		kusciaAgentConfig.ContainerLogMaxFiles = overwriteAgentLogrotate.ContainerLogMaxFiles
	} else if overwriteLogrotate != nil && overwriteLogrotate.MaxFiles > 1 {
		kusciaAgentConfig.ContainerLogMaxFiles = overwriteLogrotate.MaxFiles
	}
	if overwriteAgentLogrotate != nil {
		if _, parseErr := parseMaxSize(overwriteAgentLogrotate.ContainerLogMaxSize); parseErr == nil {
			kusciaAgentConfig.ContainerLogMaxSize = overwriteAgentLogrotate.ContainerLogMaxSize
			return
		}
	}
	if overwriteLogrotate != nil && overwriteLogrotate.MaxFileSizeMB > 0 {
		kusciaAgentConfig.ContainerLogMaxSize = fmt.Sprintf("%dMi", overwriteLogrotate.MaxFileSizeMB)
	}

	if kusciaAgentConfig.ContainerLogMaxFiles <= 0 {
		kusciaAgentConfig.ContainerLogMaxFiles = config.DefaultLogRotateMaxFiles
	}

	if kusciaAgentConfig.ContainerLogMaxSize == "" {
		kusciaAgentConfig.ContainerLogMaxSize = config.DefaultLogRotateMaxSizeStr
	}
}

// try to overwrite kuscia logrotate default config with kuscia yaml logrotate config
func overwriteKusciaConfigLogrotate(kusciaConfig, overwriteLogrotate *LogrotateConfig) {
	if overwriteLogrotate != nil {
		if overwriteLogrotate.MaxFileSizeMB > 0 {
			kusciaConfig.MaxFileSizeMB = overwriteLogrotate.MaxFileSizeMB
		}
		if overwriteLogrotate.MaxFiles > 0 {
			kusciaConfig.MaxFiles = overwriteLogrotate.MaxFiles
		}
		if overwriteLogrotate.MaxAgeDays > 0 {
			kusciaConfig.MaxAgeDays = overwriteLogrotate.MaxAgeDays
		}
	}
}

func loadConfig(configFile string, conf interface{}) error {
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(content, conf); err != nil {
		return err
	}
	return nil
}

func GenerateCsrData(domainID, domainKeyData, deployToken string) string {
	if domainKeyData == "" {
		nlog.Fatalf("Domain key data is empty. Please provide a valid domainKeyData.")
	}
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
		id, convErr := strconv.Atoi(str)
		if convErr != nil {
			nlog.Fatalf("Parse extension ID error: %v", convErr.Error())
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

// copied from kubelet/log/container_log_manager
func parseMaxSize(size string) (int64, error) {
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return 0, err
	}
	maxSize, ok := quantity.AsInt64()
	if !ok {
		return 0, fmt.Errorf("invalid max log size")
	}
	return maxSize, nil
}
