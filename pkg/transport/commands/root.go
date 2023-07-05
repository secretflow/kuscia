// Copyright 2023 Ant Group Co., Ltd.
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

package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/pkg/transport/server/http"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
)

func NewCommand(opts *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "transport",
		Long:         "transport data for interconnection with third party privacy computing platform",
		Version:      meta.KusciaVersionString(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			zLogWriter, err := zlogwriter.New(opts.logCfg)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zLogWriter))

			err = http.Run(context.Background(), opts.configFile)
			if err != nil {
				nlog.Fatalf("Failed to start transport, %v", err)
			}
			return err
		},
	}

	return cmd
}
