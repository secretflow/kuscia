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
package lite

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
)

var (
	defaultEndpoint = "http://apiserver.master.svc"
)

func getInitConfig(configFile, domainID string) *modules.Dependencies {
	content, err := os.ReadFile(configFile)
	if err != nil {
		nlog.Fatal(err)
	}
	conf := &modules.Dependencies{}
	conf.ApiserverEndpoint = defaultEndpoint
	err = yaml.Unmarshal(content, &conf.KusciaConfig)
	if err != nil {
		nlog.Fatal(err)
	}
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	conf.EnvoyIP = hostIP

	// use the current context in kubeconfig
	clients, err := kubeconfig.CreateClientSetsFromKubeconfig("", conf.ApiserverEndpoint)
	if err != nil {
		nlog.Fatal(err)
	}
	conf.Clients = clients
	err = modules.EnsureDir(conf)
	if err != nil {
		nlog.Fatal(err)
	}
	conf.ContainerdSock = filepath.Join(conf.RootDir, "containerd/run/containerd.sock")
	conf.ExternalTLS = &kusciaconfig.TLSConfig{
		CertFile: filepath.Join(conf.RootDir, modules.CertPrefix, "external_tls.crt"),
		KeyFile:  filepath.Join(conf.RootDir, modules.CertPrefix, "external_tls.key"),
		CAFile:   conf.CAFile,
	}
	return conf
}

func NewLiteCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	domainID := ""
	debug := false
	debugPort := 28080
	var logConfig *zlogwriter.LogConfig
	cmd := &cobra.Command{
		Use:          "lite",
		Short:        "Lite means only running as a node",
		Long:         `Lite contains node modules, such as: agent, envoy, domainroute, coredns, containerd`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runCtx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
			}()
			zlog, err := zlogwriter.New(logConfig)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zlog))
			conf := getInitConfig(configFile, domainID)
			_, _, err = modules.EnsureCaKeyAndCert(conf)
			if err != nil {
				nlog.Error(err)
				return err
			}
			err = modules.EnsureDomainKey(conf)
			if err != nil {
				nlog.Error(err)
				return err
			}

			modules.RunContainerd(runCtx, cancel, conf)
			modules.RunCoreDNS(runCtx, cancel, conf)
			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				modules.RunDomainRoute(runCtx, cancel, conf)
			}()
			go func() {
				defer wg.Done()
				modules.RunEnvoy(runCtx, cancel, conf)
			}()
			wg.Wait()

			modules.RunAgent(runCtx, cancel, conf)
			if debug {
				utils.SetupPprof(debugPort)
			}
			<-runCtx.Done()
			return nil
		},
	}
	cmd.Flags().StringVarP(&configFile, "conf", "c", "/home/kuscia/etc/kuscia.yaml", "config path")
	cmd.Flags().StringVarP(&domainID, "domain", "d", "", "domain id")
	cmd.Flags().BoolVar(&debug, "debug", false, "debug mode")
	cmd.Flags().IntVar(&debugPort, "debugPort", 28080, "debug mode listen port")
	logConfig = zlogwriter.InstallPFlags(cmd.Flags())
	return cmd
}
