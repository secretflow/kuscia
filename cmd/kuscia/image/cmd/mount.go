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

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func mountCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	mountCmd := &cobra.Command{
		Use:                   "mount MOUNT_ID IMAGE TARGET_PATH",
		Short:                 "mount a image to path",
		DisableFlagsInUseLine: true,
		Example: `
# Mount image to dir
kuscia image mount unique_id app:0.1 target_dir/
`,
		Args: cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.ImageService.MountImage(); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	return mountCmd
}
