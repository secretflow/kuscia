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
	"strings"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/autonomy"
	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/lite"
	"github.com/secretflow/kuscia/cmd/kuscia/master"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewStartCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	onlyControllers := false
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start means running Kuscia",
		Long:         `Start Kuscia with multi-mode from config file, Lite, Master or Autonomy`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			commonConfig := confloader.LoadCommonConfig(configFile)
			mode := strings.ToLower(commonConfig.Mode)

			if commonConfig.DomainID == common.UnSupportedDomainID {
				nlog.Fatalf("Domain id can't be 'master', please check input config file(%s)", configFile)
			}

			switch mode {
			case common.RunModeLite:
				lite.Run(ctx, configFile)
			case common.RunModeMaster:
				master.Run(ctx, configFile, onlyControllers)
			case common.RunModeAutonomy:
				autonomy.Run(ctx, configFile, onlyControllers)
			default:
				nlog.Fatalf("Unsupported mode: %s", commonConfig.Mode)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", "etc/config/kuscia.yaml", "load config from file")
	cmd.Flags().BoolVar(&onlyControllers, "controllers", false, "only run controllers and scheduler, will remove later")
	return cmd
}
