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
	"fmt"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/app/server"
	"github.com/secretflow/kuscia/pkg/diagnose/mods"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"github.com/spf13/cobra"
)

func NewDiagnoseCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diagnose",
		Short:        "Diagnose means pre-test Kuscia",
		SilenceUsage: true,
	}
	cmd.AddCommand(NewCDRCommand(ctx))
	cmd.AddCommand(NewNeworkCommand(ctx))
	cmd.AddCommand(NewAppCommand(ctx))
	return cmd
}

func NewCDRCommand(ctx context.Context) *cobra.Command {
	param := new(mods.DiagnoseConfig)
	cmd := &cobra.Command{
		Use:          modules.SubCMDClusterDomainRoute,
		Short:        "Diagnose the effectiveness of domain route",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			param.Source, param.Destination = args[0], args[1]
			param.Command = modules.SubCMDClusterDomainRoute
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

	cmd.Flags().BoolVarP(&param.Manual, "manual", "m", false, "Initialize server/client manually")
	cmd.Flags().StringVarP(&param.PeerEndpoint, "endpoint", "e", "", "Peer Endpoint, only effective in manual mode")
	return cmd
}

func NewNeworkCommand(ctx context.Context) *cobra.Command {
	param := new(mods.DiagnoseConfig)
	cmd := &cobra.Command{
		Use:          modules.SubCMDNetwork,
		Short:        "Diagnose the status of network between domains",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			param.Source, param.Destination = args[0], args[1]
			param.Command = modules.SubCMDNetwork
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
	cmd.Flags().BoolVarP(&param.Manual, "manual", "m", false, "Initialize server/client manually")
	cmd.Flags().StringVarP(&param.PeerEndpoint, "endpoint", "e", "", "Peer Endpoint, only effective in manual mode")
	return cmd
}

func RunToolCommand(ctx context.Context, config *mods.DiagnoseConfig) error {
	fmt.Printf("Diagnose Config:\n%v\n", config)
	m := modules.NewDiagnose(config)
	m.Run(ctx)
	return nil
}

func NewAppCommand(ctx context.Context) *cobra.Command {
	configFile := ""
	cmd := &cobra.Command{
		Use:          modules.SubCMDAPP,
		Short:        "Mock appliction for network diagnose",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAppCommand(ctx, configFile)
		},
	}
	cmd.Flags().StringVarP(&configFile, "conf", "c", "/etc/kuscia/task-config.conf", "config path")

	return cmd
}

func RunAppCommand(ctx context.Context, configFile string) error {
	nlog.Infof("Run net stat, configFile: %v", configFile)
	// parse config file
	config, err := ParseConfig(configFile)
	if err != nil {
		return err
	}
	nlog.Infof("Net stat config: %+v", config)

	// init server
	nlog.Infof("Init server")
	diagnoseServer := server.NewHTTPServerBean(config.ServerPort)
	go diagnoseServer.Run(ctx)

	// init client
	var needRegister bool
	// if the node doesn't know its peer's endpoint, wait for peer to register the endpint.
	// otherwise the node resgisters its own endpoint to peer in manual mode.
	// this is based on the fact in manual mode that there must be one node which doesn't know its peer's endpoint due to network issue
	if config.PeerEndpoint == "" {
		config.PeerEndpoint = diagnoseServer.WaitClient(ctx)
		if config.PeerEndpoint == "" {
			return fmt.Errorf("wait client received context done")
		}
	} else {
		needRegister = config.Manual
	}
	cli := client.NewDiagnoseClient(config.PeerEndpoint)
	if needRegister {
		registerEndpointReq := new(diagnose.RegisterEndpointRequest)
		registerEndpointReq.Endpoint = config.SelfEndpoint
		if _, err := cli.RegisterEndpoint(ctx, registerEndpointReq); err != nil {
			nlog.Errorf("Register endpoint failed, %v", err)
			return err
		}
	}

	// net test
	nlog.Infof("Run netstat test")
	taskGroup := netstat.NewTaskGroup(diagnoseServer, cli, config)
	if err := taskGroup.Start(ctx); err != nil {
		nlog.Errorf("Taskgroup failed, %v", err)
		return err
	}

	// send done signal to peer
	nlog.Infof("Send done signal to peer")
	if _, err := cli.Done(ctx); err != nil {
		nlog.Warnf("Send done signal failed, the server of other party may not close, error: %v", err)
	}

	// close server
	nlog.Infof("Close apps")
	diagnoseServer.WaitClose(ctx)
	return nil
}
