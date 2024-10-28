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

	mnt "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/mount"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type chrootStarter struct {
	*exec.Cmd
	mounter mnt.Mounter
	config  *InitConfig
}

func NewChrootStarter(c *InitConfig) (Starter, error) {
	s := &chrootStarter{config: c}

	cmdLine := wrapCommand(c.CmdLine)
	s.Cmd = exec.Command(cmdLine[0], cmdLine[1:]...)
	s.Cmd.Env = c.Env

	s.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	s.Cmd.SysProcAttr.Chroot = c.Rootfs
	s.Cmd.Dir = c.WorkingDir

	if err := paths.EnsureDirectory(filepath.Join(c.Rootfs, c.WorkingDir), true); err != nil {
		return nil, err
	}

	s.Cmd.Stdout = c.LogFile
	s.Cmd.Stderr = c.LogFile

	mounter, err := mnt.NewSysMounter(c.Rootfs)
	if err != nil {
		return nil, err
	}
	s.mounter = mounter

	nlog.Infof("Build chroot cmd: %v, rootfs: %v, workingDir: %v,", s.Cmd, s.SysProcAttr.Chroot, s.Dir)

	return s, nil
}

func wrapCommand(originalCmd []string) []string {
	if len(originalCmd) == 0 {
		return nil
	}
	var originalCmdStr = fmt.Sprintf("\"%s\"", originalCmd[0])
	for i := 1; i < len(originalCmd); i++ {
		originalCmdStr += fmt.Sprintf(" \"%s\"", originalCmd[i])
	}
	return []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("source /etc/profile; source ~/.bashrc; %s", originalCmdStr),
	}
}

func (s *chrootStarter) Start() error {
	if err := mountSystemFile(s.mounter); err != nil {
		return nil
	}

	if err := mountVolumes(s.config.Rootfs, s.mounter, s.config.ContainerConfig.Mounts); err != nil {
		return err
	}

	return s.Cmd.Start()
}

func (s *chrootStarter) Command() *exec.Cmd {
	return s.Cmd
}

func (s *chrootStarter) Release() error {
	return s.mounter.UmountRoot()
}

func (s *chrootStarter) ReopenContainerLog() error {
	return s.config.LogFile.ReopenFile()
}

func (s *chrootStarter) CloseContainerLog() error {
	return s.config.LogFile.Close()
}
