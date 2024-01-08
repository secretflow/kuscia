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

package starter

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

// PRoot is a user-space implementation of chroot, mount --bind, and binfmt_misc.
// For more information, please refer to https://github.com/proot-me/proot/blob/master/doc/proot/manual.rst#proot
type prootStarter struct {
	*exec.Cmd
}

func NewProotStarter(c *InitConfig) (Starter, error) {
	s := &prootStarter{}

	mountArgs := buildMountArgs(c.ContainerConfig.Mounts)
	cmdLine := []string{"/home/kuscia/bin/proot", "-S", c.Rootfs, "-w", c.WorkingDir}
	cmdLine = append(cmdLine, mountArgs...)
	cmdLine = append(cmdLine, c.CmdLine...)
	s.Cmd = exec.Command(cmdLine[0], cmdLine[1:]...)
	s.Cmd.Env = c.Env
	s.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := paths.EnsureDirectory(filepath.Join(c.Rootfs, c.WorkingDir), true); err != nil {
		return nil, err
	}

	s.Cmd.Stdout = c.LogFile
	s.Cmd.Stderr = c.LogFile

	nlog.Infof("Build proot cmd: %v", s.Cmd)

	return s, nil
}

func buildMountArgs(mounts []*runtime.Mount) []string {
	mountArgs := make([]string, len(mounts))
	for i, mount := range mounts {
		mountArgs[i] = fmt.Sprintf("-b %s:%s", mount.HostPath, mount.ContainerPath)
	}
	return mountArgs
}

func (s *prootStarter) Start() error {
	return s.Cmd.Start()
}

func (s *prootStarter) Command() *exec.Cmd {
	return s.Cmd
}

func (s *prootStarter) Release() error {
	return nil
}
