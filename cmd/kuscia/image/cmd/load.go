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
	"io"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// loadCommand represents the load command
func loadCommand(cmdCtx *utils.ImageContext) *cobra.Command {
	var input string

	loadCmd := &cobra.Command{
		Use:                   "load [OPTIONS]",
		Short:                 "Load an image from a tar archive or STDIN",
		DisableFlagsInUseLine: true,
		Example: `
# load image from tar archive
kuscia image load < app.tar

# load image from tar archive with gzip
kuscia image load < app.tar.gz

# load image from tar archive
kuscia image load --input app.tar
`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			tmpFile := ""
			if input == "" {
				tmpFile = path.Join("/tmp", uuid.NewString())
				file, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE, 0666)
				if err != nil {
					nlog.Fatalf("open tmp file(%s) failed with %s", tmpFile, err.Error())
				}
				defer file.Close()

				if _, err := io.Copy(file, os.Stdin); err != nil {
					nlog.Fatalf("copy stdin err: %v", err)
				}

				input = tmpFile
			}

			if tmpFile != "" {
				defer os.Remove(tmpFile)
			}

			if err := cmdCtx.ImageService.LoadImage(input); err != nil {
				nlog.Fatal(err.Error())
			}
		},
	}

	loadCmd.Flags().StringVarP(&input, "input", "i", "", "read from tar archive file, instead of STDIN")
	return loadCmd
}
