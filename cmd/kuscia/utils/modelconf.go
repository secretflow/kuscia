package utils

import (
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
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
	liteDefaultEndpoint           = "http://apiserver.master.svc"
)

func GetInitConfig(configFile string, flagDomainID string, runmodel string) *modules.Dependencies {
	content, err := os.ReadFile(configFile)
	if err != nil {
		nlog.Fatal(err)
	}
	conf := &modules.Dependencies{}
	err = yaml.Unmarshal(content, &conf.KusciaConfig)
	if err != nil {
		nlog.Fatal(err)
	}
	if conf.RootDir == "" {
		conf.RootDir = defaultRootDir
	}
	err = modules.EnsureDir(conf)
	if err != nil {
		nlog.Fatal(err)
	}
	if flagDomainID != "" {
		conf.DomainID = flagDomainID
	}
	if conf.DomainID == "" {
		conf.DomainID = defaultDomainID
	}
	conf.ApiserverEndpoint = defaultEndpoint
	if runmodel == "master" || runmodel == "autonomy" {
		conf.KubeconfigFile = filepath.Join(conf.RootDir, "etc/kubeconfig")
		conf.KusciaKubeConfig = filepath.Join(conf.RootDir, "etc/kuscia.kubeconfig")
		if conf.CAKeyFile == "" {
			conf.CAKeyFile = filepath.Join(conf.RootDir, modules.CertPrefix, "ca.key")
		}
		if conf.CAFile == "" {
			conf.CAFile = filepath.Join(conf.RootDir, modules.CertPrefix, "ca.crt")
		}
		if conf.DomainKeyFile == "" {
			conf.DomainKeyFile = filepath.Join(conf.RootDir, modules.CertPrefix, "domain.key")
		}
		conf.Master = &kusciaconfig.MasterConfig{
			APIServer: &kusciaconfig.APIServerConfig{
				KubeConfig: conf.KubeconfigFile,
				Endpoint:   conf.ApiserverEndpoint,
			},
		}
		conf.InterConnSchedulerPort = defaultInterConnSchedulerPort
	}

	if runmodel == "master" || runmodel == "lite" {
		if runmodel == "lite" {
			conf.ApiserverEndpoint = liteDefaultEndpoint
			clients, err := kubeconfig.CreateClientSetsFromKubeconfig("", conf.ApiserverEndpoint)
			if err != nil {
				nlog.Fatal(err)
			}
			conf.Clients = clients
		}
		conf.ExternalTLS = &kusciaconfig.TLSConfig{
			CertFile: filepath.Join(conf.RootDir, modules.CertPrefix, "external_tls.crt"),
			KeyFile:  filepath.Join(conf.RootDir, modules.CertPrefix, "external_tls.key"),
			CAFile:   conf.CAFile,
		}
	}

	if runmodel == "autonomy" || runmodel == "lite" {
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
	}
	return conf
}
