// Copyright 2024 Ant Group Co., Ltd.
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

package diagnose

import (
	"context"
	"path"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	dcommon "github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/diagnose/mods"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	SubCMDClusterDomainRoute = "cdr"
	SubCMDNetwork            = "network"
	SubEnvoyLogAnalysis      = "log"
)

func NewDiagnoseCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diagnose",
		Short:        "Diagnose means pre-test Kuscia",
		SilenceUsage: true,
	}
	cmd.AddCommand(NewCDRCommand(ctx))
	cmd.AddCommand(NewNetworkCommand(ctx))
	cmd.AddCommand(NewEnvoyLogCommand(ctx))
	return cmd
}

func NewCDRCommand(ctx context.Context) *cobra.Command {
	param := new(mods.DiagnoseConfig)
	param.NetworkParam = new(netstat.NetworkParam)
	cmd := &cobra.Command{
		Use:          SubCMDClusterDomainRoute,
		Short:        "Diagnose the effectiveness of domain route",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			param.Source, param.Destination = args[0], args[1]
			param.Command = SubCMDClusterDomainRoute
			return RunToolCommand(ctx, param)
		},
	}
	cmd.Flags().BoolVar(&param.Speed, "speed", true, "Enable bandwidth test")
	cmd.Flags().BoolVar(&param.RTT, "rtt", true, "Enable latency test")
	cmd.Flags().BoolVar(&param.ProxyTimeout, "proxy-timeout", false, "Enable proxy timeout test")
	cmd.Flags().BoolVar(&param.Size, "size", true, "Enable request body size test")
	cmd.Flags().BoolVar(&param.ProxyBuffer, "buffer", true, "Enable proxy buffer test")

	cmd.Flags().IntVar(&param.SpeedThres, "speed-threshold", netstat.DefaultBandWidthThreshold, "Bandwidth threshold, unit Mbits/sec")
	cmd.Flags().IntVar(&param.RTTTres, "rtt-threshold", netstat.DefaultRTTThreshold, "RTT threshold, unit ms")
	cmd.Flags().IntVar(&param.ProxyTimeoutThres, "proxy-timeout-threshold", netstat.DefaultProxyTimeoutThreshold, "Proxy timeout threshold, unit ms")
	cmd.Flags().IntVar(&param.SizeThres, "request-size-threshold", netstat.DefaultRequestBodySizeThreshold, "Request size threshold, unit MB")

	return cmd
}

func NewNetworkCommand(ctx context.Context) *cobra.Command {
	param := new(mods.DiagnoseConfig)
	param.NetworkParam = new(netstat.NetworkParam)

	cmd := &cobra.Command{
		Use:          SubCMDNetwork,
		Short:        "Diagnose the status of network between domains",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			param.Source, param.Destination = args[0], args[1]
			param.Command = SubCMDNetwork
			return RunToolCommand(ctx, param)
		},
	}
	cmd.Flags().BoolVar(&param.Speed, "speed", true, "Enable bandwidth test")
	cmd.Flags().BoolVar(&param.RTT, "rtt", true, "Enable latency test")
	cmd.Flags().BoolVar(&param.ProxyTimeout, "proxy-timeout", false, "Enable proxy timeout test")
	cmd.Flags().BoolVar(&param.Size, "size", true, "Enable request body size test")
	cmd.Flags().BoolVar(&param.ProxyBuffer, "buffer", true, "Enable proxy buffer test")

	cmd.Flags().IntVar(&param.SpeedThres, "speed-threshold", netstat.DefaultBandWidthThreshold, "Bandwidth threshold, unit Mbits/sec")
	cmd.Flags().IntVar(&param.RTTTres, "rtt-threshold", netstat.DefaultRTTThreshold, "RTT threshold, unit ms")
	cmd.Flags().IntVar(&param.ProxyTimeoutThres, "proxy-timeout-threshold", netstat.DefaultProxyTimeoutThreshold, "Proxy timeout threshold, unit ms")
	cmd.Flags().IntVar(&param.SizeThres, "request-size-threshold", netstat.DefaultRequestBodySizeThreshold, "Request size threshold, unit MB")

	cmd.Flags().BoolVarP(&param.Bidirection, "bidirection", "b", true, "Execute bidirection test")
	return cmd
}

func NewEnvoyLogCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:          SubEnvoyLogAnalysis,
		Short:        "Diagnose the task's envoy log messages",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Example: `
# analyse envoy log by taskID
kuscia diagnose log secretflow-task-20250519105140-single-psi
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunEnvoyLogCommand(ctx, args[0])
		},
	}
	return cmd
}

func RunToolCommand(ctx context.Context, config *mods.DiagnoseConfig) error {
	dir := common.DefaultKusciaHomePath()
	kusciaConf, err := confloader.ReadConfig(path.Join(dir, dcommon.KusciaYamlPath))
	if err != nil {
		nlog.Fatalf("Load kuscia config failed. %v", err.Error())
	}
	if kusciaConf.RunMode == common.RunModeMaster {
		nlog.Info("Please run kuscia diagnose in lite or autonomy mode")
		return nil
	}
	nlog.Infof("Diagnose Config:\n%v", config)
	// mod init
	reporter := util.NewReporter(config.ReportFile)
	defer func() {
		reporter.Render()
		reporter.Close()
	}()
	kusciaAPIConn, err := client.NewKusciaAPIConn()
	if err != nil {
		nlog.Fatalf("init kuscia api conn failed, %v", err)
	}
	var mod mods.Mod
	switch config.Command {
	case SubCMDNetwork:
		mod = mods.NewNetworkMod(reporter, kusciaAPIConn, config)
	case SubCMDClusterDomainRoute:
		mod = mods.NewDomainRouteMod(reporter, kusciaAPIConn, config)
	default:
		nlog.Errorf("invalid support subcmd")
		return nil
	}
	return mod.Run(ctx)
}

func RunEnvoyLogCommand(ctx context.Context, taskID string) error {
	dir := common.DefaultKusciaHomePath()
	kusciaConf, err := confloader.ReadConfig(path.Join(dir, dcommon.KusciaYamlPath))
	if err != nil {
		nlog.Fatalf("Load kuscia config failed. %v", err.Error())
	}
	if kusciaConf.RunMode == common.RunModeMaster {
		nlog.Info("Please run kuscia diagnose log in lite or autonomy mode")
		return nil
	}
	reporter := util.NewReporter("")
	defer func() {
		reporter.Render()
		reporter.Close()
	}()
	return mods.NewEnvoyLogTaskAnalysis(reporter, taskID, kusciaConf.DomainID, "", "", kusciaConf.RunMode).TaskAnalysis(ctx)
}
