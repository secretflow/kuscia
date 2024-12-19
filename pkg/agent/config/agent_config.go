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
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"os"
	"path"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	defaultLogsPath            = "var/logs"
	defaultStdoutPath          = "var/stdout"
	defaultLocalImageRootDir   = "var/images"
	defaultLocalSandboxRootDir = "sandbox"

	defaultK8sClientMaxQPS = 250
	defaultPodsCapacity    = "500"

	DefaultReservedCPU    = "0.5"
	DefaultReservedMemory = "500Mi"

	defaultCRIRemoteEndpoint = "unix:///home/kuscia/containerd/run/containerd.sock"
	defaultResolvConfig      = "/etc/resolv.conf"

	DefaultLogRotateMaxFiles   = 5
	DefaultLogRotateMaxSize    = 512
	DefaultLogRotateMaxSizeStr = "512Mi"
)

const (
	ContainerRuntime = "runc"
	K8sRuntime       = "runk"
	ProcessRuntime   = "runp"
)

type AgentLogCfg struct {
	// Defaults to INFO.
	LogLevel string `yaml:"level,omitempty"`

	// Output log to LogsPath/Filename.
	// if Filename is empty, logs will only output to stdout.
	// Backup log files will be retained in the same directory.
	Filename string `yaml:"filename,omitempty"`

	// MaxFileSizeMB is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxFileSizeMB int `yaml:"maxFileSizeMB,omitempty"`

	// MaxFiles is the maximum number of old log files to retain.  The default
	// is to retain all old log files.
	MaxFiles int `yaml:"maxFiles,omitempty"`
}

type CapacityCfg struct {
	CPU              string `yaml:"cpu"`
	Memory           string `yaml:"memory"`
	Pods             string `yaml:"pods"`
	Storage          string `yaml:"storage"`
	EphemeralStorage string `yaml:"ephemeralStorage"`
}

type ReservedResourcesCfg struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

type KubeConnCfg struct {
	KubeconfigFile string `yaml:"kubeconfigFile,omitempty"`
	Endpoint       string `yaml:"endpoint,omitempty"`
	// QPS indicates the maximum QPS to the master from this client.
	QPS float32 `yaml:"qps,omitempty"`
	// Maximum burst for throttle.
	Burst int `yaml:"burst,omitempty"`
	// The maximum length of time to wait before giving up on a server request.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

type ApiserverSourceCfg struct {
	KubeConnCfg `yaml:",inline"`
}

type FileSourceCfg struct {
	Enable bool          `yaml:"enable,omitempty"`
	Path   string        `yaml:"path,omitempty"`
	Period time.Duration `yaml:"period,omitempty"`
}

type SourceCfg struct {
	Apiserver ApiserverSourceCfg `yaml:"apiserver,omitempty"`
	File      FileSourceCfg      `yaml:"file,omitempty"`
}

type FrameworkCfg struct {
	// syncFrequency is the max period between synchronizing running
	// containers and config
	SyncFrequency time.Duration `yaml:"syncFrequency,omitempty"`
}

type CRIProviderCfg struct {
	// remoteRuntimeEndpoint is the endpoint of remote runtime service
	RemoteRuntimeEndpoint string `yaml:"remoteRuntimeEndpoint,omitempty"`
	// remoteImageEndpoint is the endpoint of remote image service
	RemoteImageEndpoint string `yaml:"remoteImageEndpoint,omitempty"`
	// RuntimeRequestTimeout is the timeout for all runtime requests except long running
	// requests - pull, logs, exec and attach.
	RuntimeRequestTimeout time.Duration `yaml:"runtimeRequestTimeout,omitempty"`
	// A quantity defines the maximum size of the container log file before it is rotated. For example: "5Mi" or "256Ki".
	ContainerLogMaxSize string `yaml:"containerLogMaxSize,omitempty"`
	// Maximum number of container log files that can be present for a container.
	ContainerLogMaxFiles int `yaml:"containerLogMaxFiles,omitempty"`
	// clusterDomain is the DNS domain for this cluster. If set, agent will
	// configure all containers to search this domain in addition to the
	// host's search domains.
	ClusterDomain string `yaml:"clusterDomain,omitempty"`
	// clusterDNS is a list of IP addresses for a cluster DNS server. If set,
	// agent will configure all containers to use this for DNS resolution
	// instead of the host's DNS servers.
	ClusterDNS []string `yaml:"clusterDNS,omitempty"`
	// ResolverConfig is the resolver configuration file used as the basis
	// for the container DNS resolution configuration.
	ResolverConfig string `yaml:"resolverConfig,omitempty"`

	LocalRuntime LocalRuntimeCfg `yaml:"localRuntime"`
}

type LocalRuntimeCfg struct {
	SandboxRootDir string `yaml:"sandboxRootDir"`
	ImageRootDir   string `yaml:"imageRootDir"`
}

// DNSCfg specifies the DNS servers and search domains of a sandbox.
type DNSCfg struct {
	// Defines how a pod's DNS will be configured. Default is None.
	Policy string `yaml:"policy,omitempty"`
	// List of DNS servers of the cluster.
	Servers []string `yaml:"servers,omitempty"`
	// List of DNS search domains of the cluster.
	Searches []string `yaml:"searches,omitempty"`
	// ResolverConfig is the resolver configuration file used as the basis
	// for the container DNS resolution configuration.
	ResolverConfig string `yaml:"resolverConfig,omitempty"`
}

type K8sProviderBackendCfg struct {
	Name   string    `yaml:"name,omitempty"`
	Config yaml.Node `yaml:"config,omitempty"`
}

type K8sProviderCfg struct {
	KubeConnCfg      `yaml:",inline"`
	Namespace        string                 `yaml:"namespace"`
	DNS              DNSCfg                 `yaml:"dns,omitempty"`
	Backend          K8sProviderBackendCfg  `yaml:"backend,omitempty"`
	LabelsToAdd      map[string]string      `yaml:"labelsToAdd,omitempty"`
	AnnotationsToAdd map[string]string      `yaml:"annotationsToAdd,omitempty"`
	AffinitiesToAdd  map[string]interface{} `yaml:"affinitiesToAdd,omitempty"`
	RuntimeClassName string                 `yaml:"runtimeClassName,omitempty"`
	EnableLogging    bool                   `yaml:"enableLogging,omitempty"`
	LogDirectory     string                 `yaml:"logDirectory,omitempty"`
	LogMaxSize       string                 `yaml:"logMaxSize,omitempty"`
	LogMaxFiles      int                    `yaml:"logMaxFiles,omitempty"`
}

type ProviderCfg struct {
	Runtime string         `yaml:"runtime,omitempty"`
	CRI     CRIProviderCfg `yaml:"cri,omitempty"`
	K8s     K8sProviderCfg `yaml:"k8s,omitempty"`
}

type NodeCfg struct {
	NodeName        string `yaml:"nodeName,omitempty"`
	EnableNodeReuse bool   `yaml:"enableNodeReuse,omitempty"`
	KeepNodeOnExit  bool   `yaml:"keepNodeOnExit,omitempty"`
}

type RegistryAuth struct {
	Repository string `yaml:"repository,omitempty"`

	SecretName string `yaml:"secretName,omitempty"`
	Username   string `yaml:"username,omitempty"`
	Password   string `yaml:"password,omitempty"`
	Auth       string `yaml:"auth,omitempty"`

	ServerAddress string `yaml:"serverAddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `yaml:"identityToken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry.
	RegistryToken string `yaml:"registryToken,omitempty"`
}

type RegistryCfg struct {
	Default RegistryAuth   `yaml:"default,omitempty"`
	Allows  []RegistryAuth `yaml:"allows,omitempty"`
}

type CertCfg struct {
	SigningCertFile string `yaml:"signingCertFile,omitempty"`
	SigningKeyFile  string `yaml:"signingKeyFile,omitempty"`
}

type PluginCfg struct {
	Name   string    `yaml:"name,omitempty"`
	Config yaml.Node `yaml:"config,omitempty"`
}

type AgentConfig struct {
	// Root directory of framework.
	RootDir      string `yaml:"rootDir,omitempty"`
	Namespace    string `yaml:"namespace,omitempty"`
	NodeIP       string `yaml:"NodeIP,omitempty"`
	APIVersion   string `yaml:"apiVersion,omitempty"`
	AgentVersion string `yaml:"agentVersion,omitempty"`

	// Location to check disk pressure
	DiskPressurePath string `yaml:"diskPressurePath,omitempty"`

	// If k3s is built into the node, it is a head node.
	Head bool `yaml:"head"`

	// path configuration.
	LogsPath   string `yaml:"logsPath,omitempty"`
	StdoutPath string `yaml:"stdoutPath,omitempty"`

	KusciaAPIProtocol common.Protocol
	// Todo: temporary solution for scql
	KusciaAPIToken string
	DomainKeyData  string
	DomainKey      *rsa.PrivateKey

	// CA configuration.
	DomainCACertFile string
	DomainCAKey      *rsa.PrivateKey
	DomainCACert     *x509.Certificate

	// AllowPrivileged if true, securityContext.Privileged will work for container.
	AllowPrivileged bool `yaml:"allowPrivileged,omitempty"`

	Capacity          CapacityCfg          `yaml:"capacity,omitempty"`
	ReservedResources ReservedResourcesCfg `yaml:"reservedResources"`
	Log               AgentLogCfg          `yaml:"log,omitempty"`
	Source            SourceCfg            `yaml:"source,omitempty"`
	Framework         FrameworkCfg         `yaml:"framework,omitempty"`
	Provider          ProviderCfg          `yaml:"provider,omitempty"`
	Node              NodeCfg              `yaml:"node,omitempty"`
	Registry          RegistryCfg          `yaml:"registry,omitempty"`
	Cert              CertCfg              `yaml:"cert,omitempty"`
	Plugins           []PluginCfg          `yaml:"plugins,omitempty"`
}

func DefaultStaticAgentConfig() *AgentConfig {
	return &AgentConfig{
		RootDir: common.DefaultKusciaHomePath,

		LogsPath:   defaultLogsPath,
		StdoutPath: defaultStdoutPath,

		DiskPressurePath: path.Join(common.DefaultKusciaHomePath, common.DefaultDomainDataSourceLocalFSPath),

		Capacity: CapacityCfg{
			Pods: defaultPodsCapacity,
		},
		ReservedResources: ReservedResourcesCfg{
			CPU:    DefaultReservedCPU,
			Memory: DefaultReservedMemory,
		},
		AllowPrivileged: false,
		Log: AgentLogCfg{
			LogLevel:      "INFO",
			Filename:      "",
			MaxFileSizeMB: DefaultLogRotateMaxSize,
			MaxFiles:      DefaultLogRotateMaxFiles,
		},
		Source: SourceCfg{
			Apiserver: ApiserverSourceCfg{
				KubeConnCfg: KubeConnCfg{
					QPS:   defaultK8sClientMaxQPS,
					Burst: defaultK8sClientMaxQPS * 2,
					// K8S does not set timeout by default, so the agent sets a more tolerant value by default.
					Timeout: 20 * time.Minute,
				},
			},
			File: FileSourceCfg{
				Enable: false,
			},
		},
		Framework: FrameworkCfg{
			SyncFrequency: 10 * time.Second,
		},
		Provider: ProviderCfg{
			Runtime: ContainerRuntime,
			CRI: CRIProviderCfg{
				RemoteRuntimeEndpoint: defaultCRIRemoteEndpoint,
				RemoteImageEndpoint:   defaultCRIRemoteEndpoint,
				RuntimeRequestTimeout: 2 * time.Minute,
				ContainerLogMaxSize:   DefaultLogRotateMaxSizeStr,
				ContainerLogMaxFiles:  DefaultLogRotateMaxFiles,
				ClusterDomain:         "",
				ClusterDNS:            []string{},
				ResolverConfig:        defaultResolvConfig,
				LocalRuntime: LocalRuntimeCfg{
					SandboxRootDir: defaultLocalSandboxRootDir,
					ImageRootDir:   defaultLocalImageRootDir,
				},
			},
		},
		Plugins: []PluginCfg{
			{
				Name: common.PluginNameImageSecurity,
			},
			{
				Name: common.PluginNameEnvImport,
			},
			{
				Name: common.PluginNameCertIssuance,
			},
			{
				Name: common.PluginNameConfigRender,
			},
		},
	}
}

func DefaultAgentConfig(rootDir string) *AgentConfig {
	// default agent config.
	config := DefaultStaticAgentConfig()

	if hostIP, err := network.GetHostIP(); err == nil {
		config.NodeIP = hostIP
	} else {
		nlog.Fatalf("Get host ip fail, err=%v . You should set host ip manually in config file.", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		nlog.Fatalf("Get hostname fail: %v", err)
	}
	config.StdoutPath = filepath.Join(rootDir, common.StdoutPrefix)
	if config.Node.NodeName == "" {
		config.Node.NodeName = hostname
	}
	if err := resources.ValidateK8sName(config.Node.NodeName, "node_id"); err != nil {
		nlog.Fatalf(err.Error())
	}
	return config
}

func LoadOverrideConfig(config *AgentConfig, configPath string) (*AgentConfig, error) {
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

// LoadAgentConfig loads the given json configuration files.
func LoadAgentConfig(configPath string) (*AgentConfig, error) {
	config, err := LoadOverrideConfig(DefaultAgentConfig(common.DefaultKusciaHomePath), configPath)
	if err != nil {
		return nil, err
	}

	if !filepath.IsAbs(config.LogsPath) {
		config.LogsPath = filepath.Join(config.RootDir, config.LogsPath)
	}
	if err := paths.EnsureDirectory(config.LogsPath, true); err != nil {
		return nil, err
	}

	if !filepath.IsAbs(config.StdoutPath) {
		config.StdoutPath = filepath.Join(config.RootDir, config.StdoutPath)
	}
	if err := paths.EnsureDirectory(config.StdoutPath, true); err != nil {
		return nil, err
	}

	if config.DiskPressurePath == "" {
		config.DiskPressurePath = path.Join(config.RootDir, common.DefaultDomainDataSourceLocalFSPath)
	}

	if config.NodeIP == "" {
		return nil, errors.New("empty host ip")
	}

	if len(config.Provider.CRI.ClusterDNS) == 0 {
		config.Provider.CRI.ClusterDNS = []string{config.NodeIP}
	}

	return config, nil
}

func DefaultImageStoreDir() string {
	return defaultLocalImageRootDir
}
