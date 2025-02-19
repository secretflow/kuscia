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

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/utils/logutils"
)

type InitConfig struct {
	CmdLine         []string
	Env             []string
	ContainerConfig *runtime.ContainerConfig
	Rootfs          string
	WorkingDir      string
	LogFile         *logutils.ReopenableLogger
}

type Starter interface {
	Start() error
	Command() *exec.Cmd
	Wait() error
	Release() error
	ReopenContainerLog() error
	CloseContainerLog() error
}
