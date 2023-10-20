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

//nolint:dulp
package utils

import (
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

var (
	defaultRootDir                = "/home/kuscia/"
	defaultDomainID               = "kuscia"
	defaultEndpoint               = "https://127.0.0.1:6443"
	defaultInterConnSchedulerPort = 8084
	defaultEndpointForLite        = "http://apiserver.master.svc"
)

const (
	RunModeMaster   = "master"
	RunModeAutonomy = "autonomy"
	RunModeLite     = "lite"
)

func GetInitConfig(configFile string, flagDomainID string, runmodel string) *modules.Dependencies {
	conf := &modules.Dependencies{}
	if runmodel == RunModeAutonomy || runmodel == RunModeLite {
		conf.Agent.AgentConfig = *config.DefaultAgentConfig()
	}
	if configFile != "" {
		content, err := os.ReadFile(configFile)
		if err != nil {
			nlog.Fatal(err)
		}
		err = yaml.Unmarshal(content, &conf.KusciaConfig)
		if err != nil {
			nlog.Fatal(err)
		}
	}
	if conf.RootDir == "" {
		conf.RootDir = defaultRootDir
	}
	err := modules.EnsureDir(conf)
	if err != nil {
		nlog.Fatal(err)
	}
	if flagDomainID != "" {
		conf.DomainID = flagDomainID
	}
	if conf.DomainID == "" {
		conf.DomainID = defaultDomainID
	}
	if runmodel == RunModeMaster || runmodel == RunModeAutonomy {
		conf.ApiserverEndpoint = defaultEndpoint
		conf.KubeconfigFile = filepath.Join(conf.RootDir, "etc/kubeconfig")
		conf.KusciaKubeConfig = filepath.Join(conf.RootDir, "etc/kuscia.kubeconfig")
		if conf.CAKeyFile == "" {
			conf.CAKeyFile = filepath.Join(conf.RootDir, modules.CertPrefix, "ca.key")
		}
		if conf.CACertFile == "" {
			conf.CACertFile = filepath.Join(conf.RootDir, modules.CertPrefix, "ca.crt")
		}
		if conf.DomainKeyFile == "" {
			conf.DomainKeyFile = filepath.Join(conf.RootDir, modules.CertPrefix, "domain.key")
		}
		if conf.DomainCertFile == "" {
			conf.DomainCertFile = filepath.Join(conf.RootDir, modules.CertPrefix, "domain.crt")
		}
		var APIWhitelist []string
		if conf.KusciaConfig.Master != nil {
			APIWhitelist = conf.KusciaConfig.Master.APIWhitelist
		}
		conf.Master = &kusciaconfig.MasterConfig{
			APIServer: &kusciaconfig.APIServerConfig{
				KubeConfig: conf.KubeconfigFile,
				Endpoint:   conf.ApiserverEndpoint,
			},
			APIWhitelist: APIWhitelist,
		}
		conf.InterConnSchedulerPort = defaultInterConnSchedulerPort
	}
	if runmodel == RunModeAutonomy || runmodel == RunModeLite {
		if runmodel == RunModeLite {
			if conf.ApiserverEndpoint == "" {
				conf.ApiserverEndpoint = defaultEndpointForLite
			}
			clients, err := kubeconfig.CreateClientSetsFromKubeconfig("", conf.ApiserverEndpoint)
			if err != nil {
				nlog.Fatal(err)
			}
			conf.Clients = clients
		}
		hostIP, err := network.GetHostIP()
		if err != nil {
			nlog.Fatal(err)
		}
		conf.EnvoyIP = hostIP
		conf.ContainerdSock = filepath.Join(conf.RootDir, "containerd/run/containerd.sock")
		conf.TransportConfigFile = filepath.Join(conf.RootDir, "etc/conf/transport/transport.yaml")
		conf.TransportPort, err = modules.GetTransportPort(conf.TransportConfigFile)
		if err != nil {
			nlog.Fatal(err)
		}
		conf.EnableContainerd = true
		if conf.Agent.Provider.Runtime == config.K8sRuntime {
			conf.EnableContainerd = false
		}
	}
	return conf
}
