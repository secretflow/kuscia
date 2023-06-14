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
package main

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/coredns/caddy/onevent"
	_ "github.com/coredns/coredns/plugin/acl"
	_ "github.com/coredns/coredns/plugin/any"
	_ "github.com/coredns/coredns/plugin/auto"
	_ "github.com/coredns/coredns/plugin/autopath"
	_ "github.com/coredns/coredns/plugin/bind"
	_ "github.com/coredns/coredns/plugin/bufsize"
	_ "github.com/coredns/coredns/plugin/cache"
	_ "github.com/coredns/coredns/plugin/cancel"
	_ "github.com/coredns/coredns/plugin/chaos"
	_ "github.com/coredns/coredns/plugin/debug"
	_ "github.com/coredns/coredns/plugin/dnssec"
	_ "github.com/coredns/coredns/plugin/dnstap"
	_ "github.com/coredns/coredns/plugin/erratic"
	_ "github.com/coredns/coredns/plugin/errors"
	_ "github.com/coredns/coredns/plugin/file"
	_ "github.com/coredns/coredns/plugin/forward"
	_ "github.com/coredns/coredns/plugin/health"
	_ "github.com/coredns/coredns/plugin/hosts"
	_ "github.com/coredns/coredns/plugin/loadbalance"
	_ "github.com/coredns/coredns/plugin/log"
	_ "github.com/coredns/coredns/plugin/loop"
	_ "github.com/coredns/coredns/plugin/metadata"
	_ "github.com/coredns/coredns/plugin/metrics"
	_ "github.com/coredns/coredns/plugin/nsid"
	_ "github.com/coredns/coredns/plugin/pprof"
	_ "github.com/coredns/coredns/plugin/ready"
	_ "github.com/coredns/coredns/plugin/reload"
	_ "github.com/coredns/coredns/plugin/rewrite"
	_ "github.com/coredns/coredns/plugin/root"
	_ "github.com/coredns/coredns/plugin/route53"
	_ "github.com/coredns/coredns/plugin/secondary"
	_ "github.com/coredns/coredns/plugin/sign"
	_ "github.com/coredns/coredns/plugin/template"
	_ "github.com/coredns/coredns/plugin/transfer"
	_ "github.com/coredns/coredns/plugin/whoami"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kubectlcmd "k8s.io/kubectl/pkg/cmd"

	_ "github.com/secretflow/kuscia/pkg/agent/middleware/plugins"

	"github.com/secretflow/kuscia/cmd/kuscia/autonomy"
	"github.com/secretflow/kuscia/cmd/kuscia/lite"
	"github.com/secretflow/kuscia/cmd/kuscia/master"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

func main() {
	rootCmd := &cobra.Command{
		Use:               "kuscia",
		Long:              `kuscia is a root cmd, please select subcommand you want`,
		Version:           meta.KusciaVersionString(),
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	// shecduler will register version flag when kuscia process init. clear it.
	pflag.CommandLine = nil
	ctx := signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler())
	rootCmd.AddCommand(autonomy.NewAutonomyCommand(ctx))
	rootCmd.AddCommand(lite.NewLiteCommand(ctx))
	rootCmd.AddCommand(master.NewMasterCommand(ctx))
	initKubeEnv()
	rootCmd.AddCommand(kubectlcmd.NewDefaultKubectlCommand())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initKubeEnv() {
	kubenv := os.Getenv("KUBECONFIG")
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--kubeconfig=") {
			kubenv = strings.Split(arg, "=")[1]
		} else if strings.HasPrefix(arg, "--kubeconfig") && i+1 < len(os.Args) {
			kubenv = os.Args[i+1]
		}
	}
	if kubenv == "" {
		config := "/home/kuscia/etc/kubeconfig"
		if _, err := os.Stat(config); err == nil {
			os.Setenv("KUBECONFIG", config)
		}
	}
}
