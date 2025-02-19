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
	"context"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/agent/config"
)

func NewContainerCommand(ctx context.Context) *cobra.Command {
	runtimeType := ""
	storageDir := ""
	imageCtx := &utils.ImageContext{}
	command := &cobra.Command{
		Use:          "container",
		Short:        "Manage containers",
		SilenceUsage: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			imageCtx.ImageService = utils.NewImageService(runtimeType, storageDir, args, c)
		},
	}

	command.PersistentFlags().StringVar(&storageDir, "store", config.DefaultImageStoreDir(), "kuscia image storage directory")
	command.PersistentFlags().StringVar(&runtimeType, "runtime", "", "kuscia runtime type: runp/runc")

	command.AddCommand(runCommand(imageCtx))

	return command
}
