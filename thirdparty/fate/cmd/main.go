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
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/secretflow/kuscia/pkg/utils/signals"
	fatejob "github.com/secretflow/kuscia/thirdparty/fate/cmd/job"
)

func main() {
	rootCmd := &cobra.Command{
		Use:               "fate",
		Long:              `fate is a root cmd, please select subcommand you want`,
		Version:           "0.0.1",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	pflag.CommandLine = nil
	ctx := signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler())
	rootCmd.AddCommand(fatejob.NewFateJobCommand(ctx))
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
