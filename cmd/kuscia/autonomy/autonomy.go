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
package autonomy

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/spf13/cobra"
)

func NewAutonomyCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	domainID := ""
	debug := false
	debugPort := 28080
	onlyControllers := false
	var logConfig *nlog.LogConfig
	cmd := &cobra.Command{
		Use:          "autonomy",
		Short:        "Autonomy contains all modules",
		Long:         `Autonomy contains all modules, such as: k3s, controllers, scheduler, agent, domainroute, envoy and so on`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runctx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
			}()

			err := modules.InitLogs(logConfig)
			if err != nil {
				fmt.Println(err)
				return err
			}

			kusciaConf := confloader.ReadConfig(configFile, domainID, common.RunModeAutonomy)
			nlog.Debugf("Read kuscia config: %+v", kusciaConf)

			// dns must start before dependencies because that dependencies init process may access network.
			var coreDnsModule modules.Module
			if !onlyControllers {
				coreDnsModule = modules.RunCoreDNS(runctx, cancel, &kusciaConf)
			}

			conf := modules.InitDependencies(ctx, kusciaConf)

			conf.LogConfig = logConfig
			err = modules.LoadCaDomainKeyAndCert(conf)
			if err != nil {
				nlog.Error(err)
				return err
			}
			err = modules.EnsureDomainCert(conf)
			if err != nil {
				nlog.Error(err)
				return err
			}
			if onlyControllers {
				conf.MakeClients()
				modules.RunOperatorsAllinOne(runctx, cancel, conf, true)

				nlog.Info("Scheduler and controllers are all started")
				// wait any controller failed
			} else {
				modules.RunK3s(runctx, cancel, conf)
				// make clients after k3s start
				conf.MakeClients()

				cdsModule, ok := coreDnsModule.(*modules.CorednsModule)
				if !ok {
					return errors.New("coredns module type is invalid")
				}
				cdsModule.StartControllers(runctx, conf.Clients.KubeClient)

				if err = modules.CreateDefaultDomain(ctx, conf); err != nil {
					nlog.Error(err)
					return err
				}

				if conf.EnableContainerd {
					modules.RunContainerd(runctx, cancel, conf)
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
