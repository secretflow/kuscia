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

package container

import (
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// runCommand represents the pull command
func runCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	cname := uuid.NewString()
	runCmd := &cobra.Command{
		Use:                   "run [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short:                 "Create and run a new container from an image",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		Example: `
# run image
kuscia container run secretflow/secretflow:latest bash

`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.ImageService.ImageRun(cname); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	runCmd.Flags().StringVarP(&cname, "name", "", "test", "container name")

	return runCmd
}
