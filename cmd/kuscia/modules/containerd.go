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

package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type containerdModule struct {
	Socket string
	Root   string
}

func NewContainerd(i *Dependencies) Module {
	return &containerdModule{
		Root:   i.RootDir,
		Socket: i.ContainerdSock,
	}
}

func (s *containerdModule) Run(ctx context.Context) error {
	configPath := filepath.Join(s.Root, ConfPrefix, "containerd.toml")
	configPathTmpl := filepath.Join(s.Root, ConfPrefix, "containerd.toml.tmpl")
	if err := RenderConfig(configPathTmpl, configPath, s); err != nil {
		return err
	}

	args := []string{
		"-c",
		configPath,
	}

	sp := supervisor.NewSupervisor("containerd", nil, -1)
	fout, err := os.OpenFile(filepath.Join(s.Root, LogPrefix, "containerd.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		nlog.Warnf("open containerd stdout logfile failed with %v", err)
		return nil
	}
	defer fout.Close()
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(s.Root, "bin/containerd"), args...)
		cmd.Stderr = fout
		cmd.Stdout = fout

		return cmd
	})
}

func (s *containerdModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	tickerReady := time.NewTicker(time.Second)
	for {
		select {
		case <-tickerReady.C:
			if _, err := os.Stat(s.Socket); err == nil {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			return fmt.Errorf("wait containerd ready timeout")
		}
	}
}

func (s *containerdModule) Name() string {
	return "containerd"
}

func RunContainerd(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewContainerd(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Error(err)
		cancel()
	} else {
		nlog.Info("containerd is ready")
	}
	return m
}
