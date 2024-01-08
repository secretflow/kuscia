// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// builtinCommand represents the builtin command
func builtinCommand(cmdCtx *Context) *cobra.Command {
	var manifestFile string

	registerCmd := &cobra.Command{
		Use:                   "builtin [OPTIONS]",
		Short:                 "Load a built-in image",
		DisableFlagsInUseLine: true,
		Example: `
# load built-in image
kuscia image builtin secretflow:latest
`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.Store.RegisterImage(args[0], manifestFile); err != nil {
				nlog.Fatal(err)
			}
		},
	}

	registerCmd.Flags().StringVarP(&manifestFile, "manifest", "m", "", "image manifest file")
	return registerCmd
}
