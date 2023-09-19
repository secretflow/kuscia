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
package master

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

var (
	defaultEndpoint = "https://127.0.0.1:6443"
	defaultRootDir  = "/home/kuscia/"

	defaultInterConnSchedulerPort = 8084
)

func getInitConfig(configFile string, flagDomainID string) *modules.Dependencies {
	content, err := os.ReadFile(configFile)
	if err != nil {
		nlog.Fatal(err)
	}
	conf := &modules.Dependencies{}
	err = yaml.Unmarshal(content, &conf.KusciaConfig)
	if err != nil {
		nlog.Fatal(err)
	}

	if flagDomainID != "" {
		conf.DomainID = flagDomainID
	}
	if conf.RootDir == "" {
		conf.RootDir = defaultRootDir
	}
	conf.ApiserverEndpoint = defaultEndpoint
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
		APIWhitelist: conf.KusciaConfig.Master.APIWhitelist,
	}
	err = modules.EnsureDir(conf)
	if err != nil {
		nlog.Fatal(err)
	}
	conf.InterConnSchedulerPort = defaultInterConnSchedulerPort
	return conf
}

func NewMasterCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	domainID := ""
	debug := false
	debugPort := 28080
	onlyControllers := false
	var logConfig *zlogwriter.LogConfig
	cmd := &cobra.Command{
		Use:          "master",
		Short:        "Master means only running as master",
		Long:         `Master contains master modules, such as: k3s, domainroute, envoy, controllers, scheduler`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runctx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
			}()
			zlog, err := zlogwriter.New(logConfig)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zlog))
			logs.Setup(nlog.SetWriter(zlog))
			conf := getInitConfig(configFile, domainID)
			conf.IsMaster = true

			if onlyControllers {
				// only for demo, remove later
				// use the current context in kubeconfig
				if conf.Clients, err = kubeconfig.CreateClientSetsFromKubeconfig(conf.KubeconfigFile, conf.ApiserverEndpoint); err != nil {
					nlog.Error(err)
					return err
				}

				modules.RunOperatorsAllinOne(runctx, cancel, conf, false)

				nlog.Info("Scheduler and controllers are all started")
				// wait any controller failed
			} else {
				modules.RunK3s(runctx, cancel, conf)
				modules.RunConfManager(runctx, cancel, conf)

				// use the current context in kubeconfig
				clients, err := kubeconfig.CreateClientSetsFromKubeconfig(conf.KubeconfigFile, conf.ApiserverEndpoint)
				if err != nil {
					nlog.Error(err)
					return err
				}
				conf.Clients = clients
				if err = createDefaultNamespace(ctx, conf); err != nil {
					nlog.Error(err)
					return err
				}

				wg := sync.WaitGroup{}
				wg.Add(2)
				go func() {
					defer wg.Done()
					modules.RunOperatorsInSubProcess(runctx, cancel)
				}()
				go func() {
					defer wg.Done()
					modules.RunEnvoy(runctx, cancel, conf)
				}()
				wg.Wait()

				if debug {
					utils.SetupPprof(debugPort)
				}
			}

			<-runctx.Done()
			return nil
		},
	}
	cmd.Flags().StringVarP(&configFile, "conf", "c", "", "config path")
	cmd.Flags().StringVarP(&domainID, "domain", "d", "", "domain id")
	cmd.Flags().BoolVar(&debug, "debug", false, "debug mode")
	cmd.Flags().IntVar(&debugPort, "debugPort", 28080, "debug mode listen port")
	cmd.Flags().BoolVar(&onlyControllers, "controllers", false, "only run controllers and scheduler, will remove later")
	logConfig = zlogwriter.InstallPFlags(cmd.Flags())
	return cmd
}

func createDefaultNamespace(ctx context.Context, conf *modules.Dependencies) error {
	nlog.Infof("create domain namespace %s for master", conf.DomainID)
	_, err := conf.Clients.KubeClient.CoreV1().Namespaces().Create(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: conf.DomainID}}, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}
