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

	"golang.org/x/sys/unix"

	"github.com/secretflow/kuscia/pkg/agent/commands"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/embedstrings"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	k8sVersion = "v0.26.3"
)

type agentModule struct {
	conf    *config.AgentConfig
	clients *kubeconfig.KubeClients
}

func NewAgent(i *ModuleRuntimeConfigs) (Module, error) {
	conf := &i.Agent
	conf := &i.Agent
	conf.RootDir = i.RootDir
	conf.Namespace = i.DomainID
	hostname, err := os.Hostname()
	if err != nil {
		nlog.Fatalf("Get hostname fail: %v", err)
	}
	conf.StdoutPath = filepath.Join(i.RootDir, common.StdoutPrefix)
	if conf.Node.NodeName == "" {
		conf.Node.NodeName = hostname
	}
	if conf.Provider.Runtime == config.ContainerRuntime {
		conf.Node.KeepNodeOnExit = true
	}
	if conf.Provider.Runtime == config.ProcessRuntime {
		precheckKernelVersion()
	}
	conf.APIVersion = k8sVersion
	conf.AgentVersion = fmt.Sprintf("%v", meta.AgentVersionString())
	conf.DomainKey = i.DomainKey
	conf.DomainCACert = i.CACert
	conf.DomainCAKey = i.CAKey
	conf.DomainCACertFile = i.CACertFile
	if !path.IsAbs(conf.DomainCACertFile) {
		conf.DomainCACertFile = filepath.Join(i.RootDir, i.CACertFile)
	}
	conf.AllowPrivileged = i.Agent.AllowPrivileged
	conf.Provider.CRI.RemoteImageEndpoint = fmt.Sprintf("unix://%s", i.ContainerdSock)
	conf.Provider.CRI.RemoteRuntimeEndpoint = fmt.Sprintf("unix://%s", i.ContainerdSock)

	if i.Image != nil && len(i.Image.Registries) > 0 {
		defaultRegIdx := 0 // use first registry as default registry
		for idx, reg := range i.Image.Registries {
			nlog.Infof("registry(%s), endpoint(%s)", reg.Name, reg.Endpoint)
			conf.Registry.Allows = append(conf.Registry.Allows, config.RegistryAuth{
				Repository: reg.Endpoint,
				Username:   reg.UserName,
				Password:   reg.Password,
			})

			if i.Image.DefaultRegistry == reg.Name {
				defaultRegIdx = idx
			}
		}

		conf.Registry.Default = conf.Registry.Allows[defaultRegIdx]
	} else { // deprecated. remove it 1year later
		conf.Registry.Default.Repository = os.Getenv("REGISTRY_ENDPOINT")
		conf.Registry.Default.Username = os.Getenv("REGISTRY_USERNAME")
		conf.Registry.Default.Password = os.Getenv("REGISTRY_PASSWORD")
	}
	conf.Plugins = i.Agent.Plugins

	conf.KusciaAPIProtocol = i.KusciaAPI.Protocol
	// Todo: temporary solution for scql
	if i.KusciaAPI != nil && i.KusciaAPI.Token != nil {
		token, err := utils.ReadToken(*i.KusciaAPI.Token)
		if err != nil {
			nlog.Fatalf("Parse kuscia api token fail for scql: %v.", err)
		}
		conf.KusciaAPIToken = token
	}
	conf.DomainKeyData = i.DomainKeyData

	nlog.Debugf("Agent config is %+v", conf)

	return &agentModule{
		conf:    conf,
		clients: i.Clients,
	}, nil
}

func precheckKernelVersion() {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		nlog.Warnf("Get kernel version fail: %v", err)
		return
	}
	var kernel, major uint64
	release := unix.ByteSliceToString(uts.Release[:])
	_, err := fmt.Sscanf(release, "%d.%d", &kernel, &major)
	if err != nil {
		nlog.Warnf("Failed to parse kernel version %s: %v", release, err)
		return
	}
	if kernel < 4 || kernel == 4 && major < 8 {
		nlog.Warnf("Kernel version < 4.8, set PROOT_NO_SECCOMP=1")
		os.Setenv("PROOT_NO_SECCOMP", "1")
	}
}

func (agent *agentModule) Run(ctx context.Context) error {
	if agent.conf.Provider.Runtime != config.K8sRuntime {
		// runc/runp both need run this
		cmd := exec.CommandContext(ctx, "sh", "-c", embedstrings.CGrouptPreDetectScript)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			nlog.Warn(err)
		}
	}

	return commands.RunRootCommand(ctx, agent.conf, agent.clients.KubeClient)
}

func (agent *agentModule) WaitReady(ctx context.Context) error {
	return WaitChannelReady(ctx, commands.ReadyChan, 60*time.Second)
}

func (agent *agentModule) Name() string {
	return "agent"
}
