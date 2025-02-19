// Copyright 2024 Ant Group Co., Ltd.
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

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// pullCommand represents the pull command
func pullCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	var creds string

	pullCmd := &cobra.Command{
		Use:                   "pull image [OPTIONS]",
		Short:                 "Pull an image from remote registry",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		Example: `
# pull image from remote registry
kuscia image pull secretflow/secretflow:latest

# pull image from remote registry with credentials
kuscia image pull --creds "name:pass" secretflow/secretflow:latest

`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.ImageService.PullImage(creds); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	pullCmd.Flags().StringVarP(&creds, "creds", "c", "", "credentials of the registry")
	return pullCmd
}
