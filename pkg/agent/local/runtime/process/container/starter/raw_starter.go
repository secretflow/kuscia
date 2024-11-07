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
	"os/exec"
	"path/filepath"
	"syscall"

	mnt "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/mount"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type rawStarter struct {
	*exec.Cmd
	mounter mnt.Mounter
	config  *InitConfig
}

func NewRawStarter(c *InitConfig) (Starter, error) {
	s := &rawStarter{config: c}

	s.Cmd = exec.Command(c.CmdLine[0], c.CmdLine[1:]...)
	s.Cmd.Env = c.Env

	s.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	s.Cmd.Dir = filepath.Join(c.Rootfs, c.WorkingDir)
	if err := paths.EnsureDirectory(s.Cmd.Dir, true); err != nil {
		return nil, err
	}

	s.Cmd.Stdout = c.LogFile
	s.Cmd.Stderr = c.LogFile

	mounter, err := mnt.NewSymlinkMounter(c.Rootfs)
	if err != nil {
		return nil, err
	}
	s.mounter = mounter

	nlog.Infof("Build raw cmd: %v, workingDir: %v,", s.Cmd, s.Dir)

	return s, nil
}

func (s *rawStarter) Start() error {
	if err := mountVolumes(s.config.Rootfs, s.mounter, s.config.ContainerConfig.Mounts); err != nil {
		return err
	}

	return s.Cmd.Start()
}

func (s *rawStarter) Command() *exec.Cmd {
	return s.Cmd
}

func (s *rawStarter) Release() error {
	return s.mounter.UmountRoot()
}

func (s *rawStarter) ReopenContainerLog() error {
	return s.config.LogFile.ReopenFile()
}

func (s *rawStarter) CloseContainerLog() error {
	return s.config.LogFile.Close()
}
