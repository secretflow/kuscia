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

// rmCommand represents the rm command
func rmCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	rmCmd := &cobra.Command{
		Use:                   "rm image [OPTIONS]",
		Short:                 "Remove local one image by imageName or imageID",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		Example: `
# rm image by imageID
kuscia image rm 1111111

# rm image by imageName
kuscia image rm my-image:latest
`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.ImageService.RemoveImage(); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	return rmCmd
}
