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

package start

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/runtime"
)

func NewStartCommand(ctx context.Context) *cobra.Command {
	configFile := ""

	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start means running Kuscia",
		Long:         `Start Kuscia with multi-mode from config file, Lite, Master or Autonomy`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Start(ctx, configFile)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "etc/config/kuscia.yaml", "load config from file")

	return cmd
}

func Start(ctx context.Context, configFile string) error {
	commonConfig := confloader.LoadCommonConfig(configFile)
	mode := strings.ToLower(commonConfig.Mode)

	if commonConfig.DomainID == common.UnSupportedDomainID {
		nlog.Fatalf("Domain id can't be 'master', please check input config file(%s)", configFile)
	}

	kusciaConf := confloader.ReadConfig(configFile, mode)
	conf := modules.NewModuleRuntimeConfigs(ctx, kusciaConf)
	defer conf.Close()

	if conf.Agent.Provider.Runtime == config.ContainerRuntime && !runtime.Permission.HasPrivileged() {
		nlog.Errorf("Runc must run with privileged mode")
		nlog.Errorf("Please run kuscia like: docker run --privileged secretflow/kuscia")
		return errors.New("permission is error")
	}

	utils.SetupPprof(conf.Debug, conf.DebugPort)
	if runtime.Permission.HasSetOOMScorePermission() {
		modules.SetKusciaOOMScore()
	}

	master, lite, autonomy := common.RunModeMaster, common.RunModeLite, common.RunModeAutonomy

	mm := NewModuleManager()
	mm.Regist("coredns", modules.NewCoreDNS, autonomy, lite, master)
	mm.Regist("k3s", modules.NewK3s, autonomy, master)
	mm.Regist("agent", modules.NewAgent, autonomy, lite)
	mm.Regist("envoy", modules.NewEnvoy, autonomy, lite, master)
	if conf.EnableContainerd {
		mm.Regist("containerd", modules.NewContainerd, autonomy, lite)
	}

	mm.Regist("config", modules.NewConfManager, autonomy, lite, master)
	mm.Regist("controllers", modules.NewControllersModule, autonomy, master)
	mm.Regist("datamesh", modules.NewDataMesh, autonomy, lite)
	mm.Regist("domainroute", modules.NewDomainRoute, autonomy, master, lite)
	mm.Regist("interconn", modules.NewInterConn, autonomy, master)
	mm.Regist("kusciaapi", modules.NewKusciaAPI, autonomy, lite, master)
	mm.Regist("metricexporter", modules.NewMetricExporter, autonomy, lite, master)
	mm.Regist("nodeexporter", modules.NewNodeExporter, autonomy, lite, master)
	mm.Regist("ssexporter", modules.NewSsExporter, autonomy, lite, master)
	mm.Regist("scheduler", modules.NewScheduler, autonomy, master)
	mm.Regist("transport", modules.NewTransport, autonomy, lite)

	mm.SetDependencies("agent", "envoy", "k3s", "kusciaapi")
	mm.SetDependencies("envoy", "k3s")

	mm.SetDependencies("controllers", "k3s")
	mm.SetDependencies("config", "k3s", "envoy", "domainroute", "controllers")
	mm.SetDependencies("datamesh", "k3s", "config", "envoy", "domainroute")
	mm.SetDependencies("domainroute", "k3s")
	mm.SetDependencies("interconn", "k3s")
	mm.SetDependencies("kusciaapi", "k3s", "config", "domainroute")
	mm.SetDependencies("scheduler", "k3s")
	mm.SetDependencies("ssexporter", "envoy")
	mm.SetDependencies("metricexporter", "agent", "envoy", "ssexporter", "nodeexporter")
	mm.SetDependencies("transport", "envoy")
	mm.SetDependencies("k3s", "coredns")

	mm.AddReadyHook(func(ctx context.Context, mdls map[string]modules.Module) error {
		nlog.Info("Start... coredns controllers")
		cdsModule, ok := mdls["coredns"].(*modules.CorednsModule)
		if ok && cdsModule != nil {
			cdsModule.StartControllers(ctx, conf.Clients.KubeClient)
			return nil

		}
		return errors.New("coredns module type is invalid")
	}, "k3s", "coredns", "envoy", "domainroute")

	err := mm.Start(ctx, mode, conf)
	nlog.Infof("Kuscia Instance [%s] shut down", commonConfig.DomainID)
	return err
}
