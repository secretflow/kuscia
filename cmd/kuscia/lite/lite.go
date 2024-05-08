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
	"errors"
	"sync"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewLiteCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	cmd := &cobra.Command{
		Use:          "lite",
		Short:        "Lite means only running as a node",
		Long:         `Lite contains node modules, such as: agent, envoy, domainroute, coredns, containerd`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(ctx, configFile)
		},
	}
	cmd.Flags().StringVarP(&configFile, "conf", "c", "etc/conf/kuscia.yaml", "config path")
	return cmd
}

func Run(ctx context.Context, configFile string) error {
	kusciaConf := confloader.ReadConfig(configFile, common.RunModeLite)
	conf := modules.InitDependencies(ctx, kusciaConf)
	defer conf.Close()

	coreDnsModule := modules.RunCoreDNSWithDestroy(conf)

	conf.MakeClients()

	if conf.EnableContainerd {
		modules.RunContainerdWithDestroy(conf)
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		modules.RunEnvoyWithDestroy(conf)
	}()
	go func() {
		defer wg.Done()
		modules.RunDomainRouteWithDestroy(conf)
	}()
	go func() {
		defer wg.Done()
		modules.RunTransportWithDestroy(conf)
	}()
	wg.Wait()

	cdsModule, ok := coreDnsModule.(*modules.CorednsModule)
	if !ok {
		return errors.New("coredns module type is invalid")
	}
	cdsModule.StartControllers(ctx, conf.Clients.KubeClient)

	modules.RunKusciaAPIWithDestroy(conf)
	modules.RunAgentWithDestroy(conf)
	modules.RunConfManagerWithDestroy(conf)
	modules.RunDataMeshWithDestroy(conf)
	modules.RunNodeExporterWithDestroy(conf)
	modules.RunSsExporterWithDestroy(conf)
	modules.RunMetricExporterWithDestroy(conf)
	utils.SetupPprof(conf.Debug, conf.DebugPort, false)
	modules.SetKusciaOOMScore()
	conf.WaitAllModulesDone(ctx.Done())
	nlog.Errorf("Lite shut down......")
	return nil
}
