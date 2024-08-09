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

package modules

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"k8s.io/klog/v2"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	trconfig "github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

type ModuleRuntimeConfigs struct {
	confloader.KusciaConfig
	CAKey                   *rsa.PrivateKey
	CACert                  *x509.Certificate
	DomainKey               *rsa.PrivateKey
	DomainCert              *x509.Certificate
	Clients                 *kubeconfig.KubeClients
	ApiserverEndpoint       string
	KubeconfigFile          string
	ContainerdSock          string
	TransportConfigFile     string
	TransportPort           int
	InterConnSchedulerPort  int
	SsExportPort            string
	NodeExportPort          string
	MetricExportPort        string
	KusciaKubeConfig        string
	EnableContainerd        bool
	SecretBackendHolder     *secretbackend.Holder
	Image                   *confloader.ImageConfig
	DomainCertByMasterValue atomic.Value // the value is <*x509.Certificate>
	LogConfig               *nlog.LogConfig
	Logrorate               confloader.LogrotateConfig
}

func (d *ModuleRuntimeConfigs) Close() {
	if d.SecretBackendHolder != nil {
		d.SecretBackendHolder.CloseAll()
	}
}

func (d *ModuleRuntimeConfigs) LoadCaDomainKeyAndCert() error {
	var err error
	config := d.KusciaConfig

	if d.CAKey, err = tlsutils.ParseEncodedKey(config.CAKeyData, config.CAKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(config.CAKeyData) == 0, config.CAKeyFile)
		return err
	}

	if config.CACertFile == "" {
		nlog.Errorf("load cert failed: ca cert must be file, ca file should not be empty")
		return err
	}

	if d.CACert, err = tlsutils.ParseCertWithGenerated(d.CAKey, config.DomainID, nil, config.CACertFile); err != nil {
		nlog.Errorf("load cert failed: file: %s", d.CACertFile)
		return err
	}

	if d.DomainKey, err = tlsutils.ParseEncodedKey(config.DomainKeyData, config.DomainKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(config.CAKeyData) == 0, config.DomainKeyFile)
		return err
	}

	if config.DomainCertFile == "" {
		nlog.Errorf("load cert failed: ca cert must be file, ca file should not be empty")
		return err
	}

	if d.DomainCert, err = tlsutils.ParseCertWithGenerated(d.DomainKey, d.DomainID, nil, config.DomainCertFile); err != nil {
		nlog.Errorf("load cert failed: file: %s", d.DomainCertFile)
		return err
	}

	return nil
}

func (d *ModuleRuntimeConfigs) EnsureDir() error {
	if err := os.MkdirAll(filepath.Join(d.RootDir, common.CertPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(d.RootDir, common.LogPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(d.RootDir, common.StdoutPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(d.RootDir, common.TmpPrefix), 0755); err != nil {
		return err
	}
	return nil
}

func InitLogs(logConfig *nlog.LogConfig) error {
	zlog, err := zlogwriter.New(logConfig)
	if err != nil {
		return err
	}
	nlog.Setup(nlog.SetWriter(zlog))
	logs.Setup(nlog.SetWriter(zlog))
	klog.SetOutput(nlog.DefaultLogger())
	klog.LogToStderr(false)
	return nil
}

func initLoggerConfig(kusciaConf confloader.KusciaConfig, logPath string) *nlog.LogConfig {
	return &nlog.LogConfig{
		LogPath:       logPath,
		LogLevel:      kusciaConf.LogLevel,
		MaxFileSizeMB: kusciaConf.Logrotate.MaxFileSizeMB,
		MaxFiles:      kusciaConf.Logrotate.MaxFiles,
		Compress:      true,
	}
}

func NewModuleRuntimeConfigs(ctx context.Context, kusciaConf confloader.KusciaConfig) *ModuleRuntimeConfigs {
	dependencies := &ModuleRuntimeConfigs{
		KusciaConfig: kusciaConf,
	}

	// init log
	logConfig := initLoggerConfig(kusciaConf, kusciaLogPath)
	if err := InitLogs(logConfig); err != nil {
		nlog.Fatal(err)
	}

	nlog.Debugf("Read kuscia config: %+v", kusciaConf)
	dependencies.LogConfig = logConfig
	dependencies.Logrorate = kusciaConf.Logrotate
	dependencies.Image = &kusciaConf.Image
	// run config loader
	dependencies.SecretBackendHolder = secretbackend.NewHolder()
	nlog.Info("Start to init all secret backends ... ")
	for _, sbc := range dependencies.SecretBackends {
		if err := dependencies.SecretBackendHolder.Init(sbc.Name, sbc.Driver, sbc.Params); err != nil {
			nlog.Fatalf("Init secret backend name=%s params=%+v failed: %s", sbc.Name, sbc.Params, err)
		}
	}
	if len(dependencies.SecretBackends) == 0 {
		nlog.Warnf("Init all secret backend but no provider found, creating default mem type")
		if err := dependencies.SecretBackendHolder.Init(common.DefaultSecretBackendName, common.DefaultSecretBackendType, map[string]any{}); err != nil {
			nlog.Fatalf("Init default secret backend failed: %s", err)
		}
	}
	nlog.Info("Finish Initializing all secret backends")

	configLoaders, err := confloader.NewConfigLoaderChain(ctx, dependencies.KusciaConfig.ConfLoaders, dependencies.SecretBackendHolder)
	if err != nil {
		nlog.Fatalf("Init config loader failed: %s", err)
	}
	if err = configLoaders.Load(ctx, &dependencies.KusciaConfig); err != nil {
		nlog.Errorf("Load config by configloader failed: %s", err)
	}

	nlog.Debugf("After config loader handle, kuscia config is %+v", dependencies.KusciaConfig)

	// make runtime dir
	err = dependencies.EnsureDir()
	if err != nil {
		nlog.Fatal(err)
	}

	// for master and autonomy, get k3s backend config.
	if dependencies.RunMode == common.RunModeMaster || dependencies.RunMode == common.RunModeAutonomy {
		dependencies.ApiserverEndpoint = dependencies.Master.APIServer.Endpoint
		dependencies.KubeconfigFile = dependencies.Master.APIServer.KubeConfig
		dependencies.KusciaKubeConfig = filepath.Join(dependencies.RootDir, "etc/kuscia.kubeconfig")
		dependencies.InterConnSchedulerPort = defaultInterConnSchedulerPort
	}
	// for autonomy and lite
	if dependencies.RunMode == common.RunModeAutonomy || dependencies.RunMode == common.RunModeLite {
		dependencies.ContainerdSock = filepath.Join(dependencies.RootDir, "containerd/run/containerd.sock")
		dependencies.TransportConfigFile = filepath.Join(dependencies.RootDir, "etc/conf/transport/transport.yaml")
		dependencies.TransportPort, err = GetTransportPort(dependencies.TransportConfigFile)
		if err != nil {
			nlog.Fatal(err)
		}
		dependencies.EnableContainerd = false
		if dependencies.Agent.Provider.Runtime == config.ContainerRuntime {
			dependencies.EnableContainerd = true
		}
	}
	if dependencies.RunMode == common.RunModeLite {
		dependencies.ApiserverEndpoint = defaultEndpointForLite
	}

	// init certs
	if err = dependencies.LoadCaDomainKeyAndCert(); err != nil {
		nlog.Fatal(err)
	}

	// init exporter ports
	dependencies.SsExportPort = "9092"
	dependencies.NodeExportPort = "9100"
	dependencies.MetricExportPort = "9091"

	if strings.ToLower(dependencies.RunMode) == common.RunModeLite {
		clients, err := kubeconfig.CreateClientSetsFromKubeconfig(dependencies.KubeconfigFile, dependencies.ApiserverEndpoint)
		if err != nil {
			nlog.Fatalf("init k3s client failed with err: %s", err.Error())
		}

		dependencies.Clients = clients
	}
	return dependencies
}

func GetTransportPort(configPath string) (int, error) {
	transConfig, err := trconfig.LoadTransConfig(configPath)
	if err != nil {
		return 0, err
	}
	return transConfig.HTTPConfig.Port, nil
}
