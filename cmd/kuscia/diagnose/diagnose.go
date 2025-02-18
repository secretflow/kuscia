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

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/mods"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	SubCMDClusterDomainRoute = "cdr"
	SubCMDNetwork            = "network"
)

func NewDiagnoseCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diagnose",
		Short:        "Diagnose means pre-test Kuscia",
		SilenceUsage: true,
	}
	cmd.AddCommand(NewCDRCommand(ctx))
	cmd.AddCommand(NewNeworkCommand(ctx))
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

func NewNeworkCommand(ctx context.Context) *cobra.Command {
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

func RunToolCommand(ctx context.Context, config *mods.DiagnoseConfig) error {
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
