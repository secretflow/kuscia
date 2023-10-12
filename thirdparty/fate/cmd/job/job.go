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

//nolint:dulp
package fatejob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	modules "github.com/secretflow/kuscia/thirdparty/fate/cmd/modlules"
)

func getInitConfig(taskConfigPath string) *modules.FateJobConfig {
	content, err := os.ReadFile(taskConfigPath)
	if err != nil {
		nlog.Fatal(err)
	}

	conf := &modules.FateJobConfig{}
	err = json.Unmarshal(content, &conf.TaskConfig)
	if err != nil {
		fmt.Println(err)
	}

	return conf
}

func NewFateJobCommand(ctx context.Context) *cobra.Command {
	taskConfigPath := ""
	clusterAddress := ""
	var logConfig *nlog.LogConfig
	cmd := &cobra.Command{
		Use:          "job",
		Short:        "Run fate kuscia job",
		Long:         "Run fate kuscia job, just like fate job",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runctx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
			}()
			zlog, err := zlogwriter.New(logConfig)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zlog))
			nlog.Infof("cluster fate address:%s", clusterAddress)
			conf := getInitConfig(taskConfigPath)
			conf.ClusterAddress = clusterAddress

			modules.RunJob(runctx, cancel, conf)
			<-runctx.Done()
			return nil
		},
	}
	cmd.Flags().StringVarP(&taskConfigPath, "conf", "c", "", "task config path")
	cmd.Flags().StringVarP(&clusterAddress, "address", "a", "", "fate cluster ip")
	logConfig = zlogwriter.InstallPFlags(cmd.Flags())
	return cmd
}
