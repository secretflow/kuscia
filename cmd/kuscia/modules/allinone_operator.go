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
package modules

import (
	"context"
	"os"
	"os/exec"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

// TODO: The definition of this function is very ugly, we need to reconsider it later
func RunOperatorsAllinOne(runctx context.Context, cancel context.CancelFunc, conf *Dependencies, startAgent bool) error {
	RunInterConn(runctx, cancel, conf)
	RunController(runctx, cancel, conf)
	RunScheduler(runctx, cancel, conf)
	RunDomainRoute(runctx, cancel, conf)
	RunKusciaAPI(runctx, cancel, conf)

	if startAgent {
		RunAgent(runctx, cancel, conf)
		RunDataMesh(runctx, cancel, conf)
		RunTransport(runctx, cancel, conf)
	}

	return nil
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
			return cmd
		})

		nlog.Infof("Controllers subprocess finished with error: %v", err)

		if err != nil {
			nlog.Warnf("Controller supervisor watch failed")
			cancel()
		}
	}()

	return nil
}
