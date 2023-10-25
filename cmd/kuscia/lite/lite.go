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
	"fmt"
	"sync"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/spf13/cobra"
)

func NewLiteCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	domainID := ""
	debug := false
	debugPort := 28080
	var logConfig *nlog.LogConfig
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
			err := modules.InitLogs(logConfig)
			if err != nil {
				fmt.Println(err)
				return err
			}
			conf := utils.GetInitConfig(configFile, domainID, utils.RunModeLite)
			conf.LogConfig = logConfig
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

			if conf.EnableContainerd {
				modules.RunContainerd(runCtx, cancel, conf)
			}
			coreDnsModule := modules.RunCoreDNS(runCtx, cancel, conf)

			wg := sync.WaitGroup{}
			wg.Add(3)
			go func() {
				defer wg.Done()
				modules.RunDomainRoute(runCtx, cancel, conf)
			}()
			go func() {
				defer wg.Done()
				modules.RunEnvoy(runCtx, cancel, conf)
			}()
			go func() {
				defer wg.Done()
				modules.RunTransport(runCtx, cancel, conf)
			}()
			wg.Wait()

			cdsModule, ok := coreDnsModule.(*modules.CorednsModule)
			if !ok {
				return errors.New("coredns module type is invalid")
			}
			cdsModule.StartControllers(runCtx, conf.Clients.KubeClient)

			modules.RunAgent(runCtx, cancel, conf)
			modules.RunConfManager(runCtx, cancel, conf)
			modules.RunDataMesh(runCtx, cancel, conf)
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
