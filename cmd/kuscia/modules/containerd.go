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
	"path"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/sys/unix"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/embedstrings"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/ljwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type containerdModule struct {
	moduleRuntimeBase
	Socket      string
	Root        string
	Snapshotter string
	LogConfig   nlog.LogConfig
}

func NewContainerd(i *ModuleRuntimeConfigs) (Module, error) {
	return &containerdModule{
		moduleRuntimeBase: moduleRuntimeBase{
			rdz: readyz.NewFuncReadyZ(func(ctx context.Context) error {
				return checkContainerdReadyZ(ctx, i.RootDir, i.ContainerdSock)
			}),
			readyTimeout: 60 * time.Second,
			name:         "containerd",
		},
		Root:        i.RootDir,
		Socket:      i.ContainerdSock,
		Snapshotter: autoDetectSnapshotter(i.RootDir),
		LogConfig:   *i.LogConfig,
	}, nil
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
	cmd := exec.CommandContext(ctx, "sh", "-c", embedstrings.IPTablesPreDetectScript)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func autoDetectSnapshotter(root string) string {
	path := path.Join(root, "containerd")
	if !paths.CheckDirExist(path) {
		path = root
	}

	if usage, err := disk.Usage(path); err == nil {
		if usage.Fstype == "" { // overlayfs `disk`` is supported now
			stat := unix.Statfs_t{}
			err := unix.Statfs(path, &stat)
			if err == nil && stat.Type == 0x794C7630 {
				usage.Fstype = "overlay"
			}
		}

		nlog.Infof("Path(%s)'s fstype is %s", path, usage.Fstype)
		if usage.Fstype == "overlay" {
			nlog.Warnf("Containerd path(%s)'s fstype is overlay, so snapshotter set to native", path)
			nlog.Warnf("Snapshotter set to native need more disk, recommend: docker run -v {volume}:%s secretflow/kuscia/containerd", path)
			return "native"
		}
	}

	return "overlayfs"
}

func checkContainerdReadyZ(ctx context.Context, root, sock string) error {
	if _, err := os.Stat(sock); err != nil {
		return err
	}
	containerdSock := filepath.Join(root, "/containerd/run/containerd.sock")
	pause := filepath.Join(root, "/pause/pause.tar")

	args := []string{
		fmt.Sprintf("-a=%s", containerdSock),
		"-n=k8s.io",
		"images",
		"import",
		pause,
	}

	cmd := exec.CommandContext(ctx, filepath.Join(root, "bin/ctr"), args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command %q, detail-> %v", cmd, err)
	}

	return nil
}
