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
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// listCommand represents the list command
func listCommand(cmdCtx *utils.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:                   "ls [OPTIONS]",
		Short:                 "List local images",
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		Aliases:               []string{"list"},
		Example: `
# list all local images
kuscia image list

`,
		Run: func(cmd *cobra.Command, args []string) {
			if cmdCtx.RuntimeType == config.ProcessRuntime {
				images, err := cmdCtx.Store.ListImage()
				if err != nil {
					nlog.Fatalf("error: %s", err.Error())
				}

				table := tablewriter.NewWriter(os.Stdout)
				table.SetHeader([]string{"REPOSITORY", "TAG", "IMAGE ID", "SIZE"})
				table.SetAutoWrapText(false)
				table.SetAutoFormatHeaders(true)
				table.SetBorder(false)
				table.SetHeaderLine(false)
				table.SetCenterSeparator("")
				table.SetColumnSeparator("")
				table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
				table.SetAlignment(tablewriter.ALIGN_LEFT)
				table.SetTablePadding("\t")
				table.SetNoWhiteSpace(true)

				for _, img := range images {
					table.Append([]string{
						img.Repository,
						img.Tag,
						img.ImageID,
						img.Size,
					})
				}

				table.Render()
			} else {
				if err := utils.RunContainerdCmd(cmd.Context(), "crictl", "--runtime-endpoint=unix:///home/kuscia/containerd/run/containerd.sock", "images", "ls"); err != nil {
					nlog.Fatal(err.Error())
				}
			}
		},
	}

	return listCmd
}
