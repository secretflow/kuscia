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

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/ljwriter"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type containerdModule struct {
	Socket    string
	Root      string
	LogConfig nlog.LogConfig
}

func NewContainerd(i *Dependencies) Module {
	return &containerdModule{
		Root:      i.RootDir,
		Socket:    i.ContainerdSock,
		LogConfig: *i.LogConfig,
	}
}

func (s *containerdModule) Run(ctx context.Context) error {
	configPath := filepath.Join(s.Root, pkgcom.ConfPrefix, "containerd.toml")
	configPathTmpl := filepath.Join(s.Root, pkgcom.ConfPrefix, "containerd.toml.tmpl")
	if err := common.RenderConfig(configPathTmpl, configPath, s); err != nil {
		return err
	}

	// check if the file /etc/crictl.yaml exists
	crictlFile := "/etc/crictl.yaml"
	if _, err := os.Stat(crictlFile); err != nil {
		if os.IsNotExist(err) {
			if err = os.Link(filepath.Join(s.Root, pkgcom.ConfPrefix, "crictl.yaml"), crictlFile); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if err := s.execPreCmds(ctx); err != nil {
		return err
	}

	args := []string{
		"-c",
		configPath,
	}

	sp := supervisor.NewSupervisor("containerd", nil, -1)
	s.LogConfig.LogPath = filepath.Join(s.Root, pkgcom.LogPrefix, "containerd.log")
	lj, _ := ljwriter.New(&s.LogConfig)
	n := nlog.NewNLog(nlog.SetWriter(lj))
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.Command(filepath.Join(s.Root, "bin/containerd"), args...)
		cmd.Stderr = n
		cmd.Stdout = n
		return &ModuleCMD{
			cmd:   cmd,
			score: &containerdOOMScore,
		}
	})
}

func (s *containerdModule) execPreCmds(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", filepath.Join(s.Root, "scripts/deploy/iptables_pre_detect.sh"))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "sh", "-c", filepath.Join(s.Root, "scripts/deploy/cgroup_pre_detect.sh"))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func (s *containerdModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-tickerReady.C:
			if _, err := os.Stat(s.Socket); err != nil {
				continue
			}
			if err := s.importPauseImage(ctx); err != nil {
				nlog.Errorf("Unable to import pause image: %v", err)
				continue
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			return fmt.Errorf("wait containerd ready timeout")
		}
	}
}

func (s *containerdModule) importPauseImage(ctx context.Context) error {
	args := []string{
		fmt.Sprintf("-a=%s/containerd/run/containerd.sock", s.Root),
		"-n=k8s.io",
		"images",
		"import",
		fmt.Sprintf("%s/pause/pause.tar", s.Root),
	}

	cmd := exec.CommandContext(ctx, filepath.Join(s.Root, "bin/ctr"), args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command %q, detail-> %v", cmd, err)
	}

	return nil
}

func (s *containerdModule) Name() string {
	return "containerd"
}

func RunContainerdWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := NewShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "containerd",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunContainerd(runCtx, cancel, conf, shutdownEntry)
}

func RunContainerd(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m := NewContainerd(conf)
	go func() {
		defer func() {
			if shutdownEntry != nil {
				shutdownEntry.RunShutdown()
			}
		}()
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Fatalf("Containerd wait ready failed: %v", err)
	}
	nlog.Info("Containerd is ready")
	return m
}
