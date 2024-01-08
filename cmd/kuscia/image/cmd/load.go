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
	"compress/gzip"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// loadCommand represents the load command
func loadCommand(cmdCtx *Context) *cobra.Command {
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
			file := os.Stdin

			var err error
			if input != "" {
				file, err = os.Open(input)
				if err != nil {
					nlog.Fatal(err)
				}
				defer file.Close()
			}

			if err := loadImage(cmdCtx, file); err != nil {
				nlog.Fatal(err)
			}
		},
	}

	loadCmd.Flags().StringVarP(&input, "input", "i", "", "read from tar archive file, instead of STDIN")
	return loadCmd
}

func loadImage(cmdCtx *Context, input io.ReadSeeker) error {
	header := make([]byte, 2)
	_, err := input.Read(header)
	if err != nil {
		return err
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var reader io.Reader
	if header[0] == 31 && header[1] == 139 {
		reader, err = gzip.NewReader(input)
		if err != nil {
			return err
		}
	} else {
		reader = input
	}

	return cmdCtx.Store.LoadImage(reader)
}
