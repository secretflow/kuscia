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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/process"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	runenv "github.com/secretflow/kuscia/pkg/utils/runtime"
)

var (
	k3sDataDirPrefix              = "var/k3s/"
	kusciaLogPath                 = "var/logs/kuscia.log"
	defaultInterConnSchedulerPort = 8084
	defaultEndpointForLite        = "http://apiserver.master.svc"
)

type ModuleReadyHook func(m Module) error

type Module interface {
	// Startup Module. this function will hold until module stoped
	Run(ctx context.Context) error

	// wait unit module is ready
	WaitReady(ctx context.Context) error

	// get module name
	Name() string
}

type moduleRuntimeBase struct {
	rdz          readyz.ReadyZ
	readyTimeout time.Duration

	name string
}

func (mr *moduleRuntimeBase) Run(ctx context.Context) error {
	return errors.New("not implement")
}

func (mr *moduleRuntimeBase) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(mr.readyTimeout)
	defer ticker.Stop()
	tickerReady := time.NewTicker(100 * time.Millisecond)
	defer tickerReady.Stop()
	var lastCheckError error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if lastCheckError = mr.rdz.IsReady(ctx); lastCheckError == nil {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait ready timeout. last check error=%v", lastCheckError)
		}
	}
}

func (mr *moduleRuntimeBase) Name() string {
	return mr.name
}

func WaitChannelReady(ctx context.Context, ch <-chan struct{}, timeout time.Duration) error {
	ticker := time.NewTicker(timeout)
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return fmt.Errorf("wait ready timeout")
	}
}

type ModuleCMD struct {
	cmd   *exec.Cmd
	score *int
}

func (c *ModuleCMD) Start() error {
	return c.cmd.Start()
}

func (c *ModuleCMD) Wait() error {
	return c.cmd.Wait()
}

func (c *ModuleCMD) Stop() error {
	if c.cmd != nil && c.cmd.Process != nil {
		// Windows doesn't support Interrupt
		if runtime.GOOS == "windows" {
			return c.cmd.Process.Signal(os.Kill)
		}
		return c.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (c *ModuleCMD) Pid() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

func (c *ModuleCMD) SetOOMScore() error {
	if c.score == nil {
		return nil
	}

	if runenv.Permission.HasSetOOMScorePermission() {
		if err := process.SetOOMScore(c.Pid(), *c.score); err != nil {
			nlog.Warnf("Set process(%d) oom_score_adj failed with %s, skip setting it", c.Pid(), err.Error())
			return err
		}
	}
	return nil
}
