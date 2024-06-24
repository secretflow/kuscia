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

//nolint:dupl
package master

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

var (
	defaultRootDir  = "/home/kuscia/"
	defaultDomainID = "kuscia"
	defaultEndpoint = "https://127.0.0.1:6443"

	defaultInterConnSchedulerPort = 8084
)

func NewMasterCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	onlyControllers := false
	cmd := &cobra.Command{
		Use:          "master",
		Short:        "Master means only running as master",
		Long:         `Master contains master modules, such as: k3s, domainroute, envoy, controllers, scheduler`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(ctx, configFile, onlyControllers)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "etc/conf/kuscia.yaml", "config path")
	cmd.Flags().BoolVar(&onlyControllers, "controllers", false, "only run controllers and scheduler, will remove later")
	return cmd
}

func Run(ctx context.Context, configFile string, onlyControllers bool) error {
	kusciaConf := confloader.ReadConfig(configFile, common.RunModeMaster)
	conf := modules.InitDependencies(ctx, kusciaConf)
	defer conf.Close()

	if onlyControllers {
		conf.MakeClients()
		modules.RunOperatorsAllinOneWithDestroy(conf)

		utils.SetupPprof(conf.Debug, conf.CtrDebugPort, true)
		nlog.Info("Scheduler and controllers are all started")
		// wait any controller failed
	} else {
		coreDNSModule := modules.RunCoreDNSWithDestroy(conf)
		if err := modules.RunK3sWithDestroy(conf); err != nil {
			nlog.Errorf("k3s start failed: %s", err)
			return err
		}
		// make clients after k3s start
		conf.MakeClients()

		cdsModule, ok := coreDNSModule.(*modules.CorednsModule)
		if !ok {
			return errors.New("coredns module type is invalid")
		}
		cdsModule.StartControllers(ctx, conf.Clients.KubeClient)

		if err := modules.CreateDefaultDomain(ctx, conf); err != nil {
			nlog.Error(err)
			return err
		}

		if err := modules.CreateCrossNamespace(ctx, conf); err != nil {
			nlog.Error(err)
			return err
		}

		wg := sync.WaitGroup{}
		wg.Add(3)
		go func() {
			defer wg.Done()
			modules.RunOperatorsInSubProcessWithDestroy(conf)
		}()
		go func() {
			defer wg.Done()
			modules.RunEnvoyWithDestroy(conf)
		}()
		go func() {
			defer wg.Done()
			modules.RunConfManagerWithDestroy(conf)
		}()
		wg.Wait()
		modules.RunKusciaAPIWithDestroy(conf)
		modules.RunNodeExporterWithDestroy(conf)
		modules.RunSsExporterWithDestroy(conf)
		modules.RunMetricExporterWithDestroy(conf)
		utils.SetupPprof(conf.Debug, conf.DebugPort, false)

		modules.SetKusciaOOMScore()
	}
	conf.WaitAllModulesDone(ctx.Done())
	return nil
}
