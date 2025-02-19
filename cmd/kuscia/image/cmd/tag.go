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

// tagCommand represents the tag command
func tagCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	tagCmd := &cobra.Command{
		Use:                   "tag SOURCE_IMAGE[:TAG] TARGET_IMAGE[:TAG]",
		Short:                 "Create a tag TARGET_IMAGE that refers to SOURCE_IMAGE",
		Args:                  cobra.ExactArgs(2),
		DisableFlagsInUseLine: true,
		Example: `
# tag image
kuscia image tag secretflow/secretflow:v1 registry.mycompany.com/secretflow/secretflow:v1

`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmdCtx.ImageService.TagImage(); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	return tagCmd
}
