package start

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/autonomy"
	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/lite"
	"github.com/secretflow/kuscia/cmd/kuscia/master"
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
			switch mode {
			case "lite":
				lite.Run(ctx, configFile)
			case "master":
				master.Run(ctx, configFile, onlyControllers)
			case "autonomy":
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
