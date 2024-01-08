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

package image

import (
	"context"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/image/cmd"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewImageCommand(ctx context.Context) *cobra.Command {
	cmdCtx := &cmd.Context{}
	command := &cobra.Command{
		Use:          "image",
		Short:        "Manage images",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			var err error
			cmdCtx.Store, err = store.NewStore(cmdCtx.StorageDir)
			if err != nil {
				nlog.Fatal(err)
			}
		},
	}

	command.PersistentFlags().StringVar(&cmdCtx.StorageDir, "store", defaultImageStoreDir(), "kuscia image storage directory")

	cmd.InstallCommands(command, cmdCtx)

	return command
}

func defaultImageStoreDir() string {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		nlog.Fatal(err)
	}

	return filepath.Join(home, ".kuscia/var/images")
}
