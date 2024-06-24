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

//nolint:dupl
package modules

import (
	"context"
	"os"
	"os/exec"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

// TODO: The definition of this function is very ugly, we need to reconsider it later
func RunOperatorsAllinOneWithDestroy(conf *Dependencies) {
	RunInterConnWithDestroy(conf)
	RunControllerWithDestroy(conf)
	RunSchedulerWithDestroy(conf)
	RunDomainRouteWithDestroy(conf)
}

func RunOperatorsInSubProcessWithDestroy(conf *Dependencies) error {
	runCtx, cancel := context.WithCancel(context.Background())
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:      "operators",
		DestroyCh: runCtx.Done(),
		DestroyFn: cancel,
	})
	return RunOperatorsInSubProcess(runCtx, cancel)
}

func RunOperatorsInSubProcess(ctx context.Context, cancel context.CancelFunc) error {
	sp := supervisor.NewSupervisor("controllers", nil, -1)
	go func() {
		err := sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
			// fork a new process to run scheduler and controllers
			path, err := os.Executable()
			if err != nil {
				nlog.Warnf("Get executable path failed")
				return nil
			}
			args := append(os.Args[1:], "--controllers")
			nlog.Infof("Subprocess args: %v", args)
			cmd := exec.CommandContext(ctx, path, args...)
			cmd.Env = os.Environ()
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return &ModuleCMD{
				cmd:   cmd,
				score: &controllersOOMScore,
			}
		})

		nlog.Infof("Controllers subprocess finished with error: %v", err)

		if err != nil {
			nlog.Warnf("Controller supervisor watch failed")
			cancel()
		}
	}()

	return nil
}
