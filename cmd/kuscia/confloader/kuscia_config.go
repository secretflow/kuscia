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
	ClusterToken      string `yaml:"clusterToken"`
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

func (runk RunkConfig) convert2K8sProviderCfg() (k8s config.K8sProviderCfg) {
	k8s.Namespace = runk.Namespace
	k8s.KubeconfigFile = runk.KubeconfigFile
	k8s.DNS.Servers = runk.DNSServers
	return
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
	Mode          string `yaml:"mode"`
	DomainID      string `yaml:"domainID"`
	DomainKeyData string `yaml:"domainKeyData"`
	LogLevel      string `yaml:"logLevel"`
}

type AdvancedConfig struct {
	ConfLoaders    []ConfigLoaderConfig      `yaml:"confLoaders,omitempty"`
	SecretBackends []SecretBackendConfig     `yaml:"secretBackends,omitempty"`
	KusciaAPI      *kaconfig.KusciaAPIConfig `yaml:"kusciaAPI,omitempty"`
	DataMesh       *dmconfig.DataMeshConfig  `yaml:"dataMesh,omitempty"`
	DomainRoute    DomainRouteConfig         `yaml:"domainRoute,omitempty"`
	Agent          config.AgentConfig        `yaml:"agent,omitempty"`
	Protocol       common.Protocol           `yaml:"protocol"`
	Debug          bool                      `yaml:"debug"`
	DebugPort      int                       `yaml:"debugPort"`
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
	kusciaConfig.Agent.Provider.K8s = lite.Runk.convert2K8sProviderCfg()
	kusciaConfig.Agent.Provider.K8s.Backend = lite.Agent.Provider.K8s.Backend
	kusciaConfig.Agent.Provider.K8s.LabelsToAdd = lite.Agent.Provider.K8s.LabelsToAdd
	kusciaConfig.Agent.Provider.K8s.AnnotationsToAdd = lite.Agent.Provider.K8s.AnnotationsToAdd
	kusciaConfig.Agent.Capacity = lite.Capacity
	if len(lite.Agent.Plugins) > 0 {
		kusciaConfig.Agent.Plugins = lite.Agent.Plugins
	}
	kusciaConfig.Master.Endpoint = lite.MasterEndpoint
	kusciaConfig.DomainRoute.DomainCsrData = generateCsrData(lite.DomainID, lite.DomainKeyData, lite.LiteDeployToken)
	kusciaConfig.Debug = lite.Debug
	kusciaConfig.DebugPort = lite.DebugPort
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
}

func (autonomy *AutomonyKusciaConfig) OverwriteKusciaConfig(kusciaConfig *KusciaConfig) {
	kusciaConfig.LogLevel = autonomy.LogLevel
	kusciaConfig.DomainID = autonomy.DomainID
	kusciaConfig.CAKeyData = autonomy.DomainKeyData
	kusciaConfig.DomainKeyData = autonomy.DomainKeyData
	kusciaConfig.Agent.AllowPrivileged = autonomy.Agent.AllowPrivileged
	kusciaConfig.Agent.Provider.Runtime = autonomy.Runtime
	kusciaConfig.Agent.Provider.K8s = autonomy.Runk.convert2K8sProviderCfg()
	kusciaConfig.Agent.Provider.K8s.Backend = autonomy.Agent.Provider.K8s.Backend
	kusciaConfig.Agent.Provider.K8s.LabelsToAdd = autonomy.Agent.Provider.K8s.LabelsToAdd
	kusciaConfig.Agent.Provider.K8s.AnnotationsToAdd = autonomy.Agent.Provider.K8s.AnnotationsToAdd
	kusciaConfig.Agent.Capacity = autonomy.Capacity
	if len(autonomy.Agent.Plugins) > 0 {
		kusciaConfig.Agent.Plugins = autonomy.Agent.Plugins
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
