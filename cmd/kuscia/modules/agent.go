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

	"github.com/secretflow/kuscia/pkg/agent/commands"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
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

func NewAgent(i *Dependencies) Module {
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
	conf.APIVersion = k8sVersion
	conf.AgentVersion = fmt.Sprintf("%v", meta.AgentVersionString())
	conf.DomainCACert = i.CACert
	conf.DomainCAKey = i.CAKey
	conf.DomainCACertFile = i.CACertFile
	if !path.IsAbs(conf.DomainCACertFile) {
		conf.DomainCACertFile = filepath.Join(i.RootDir, i.CACertFile)
	}
	conf.AllowPrivileged = i.Agent.AllowPrivileged
	conf.Provider.CRI.RemoteImageEndpoint = fmt.Sprintf("unix://%s", i.ContainerdSock)
	conf.Provider.CRI.RemoteRuntimeEndpoint = fmt.Sprintf("unix://%s", i.ContainerdSock)
	conf.Registry.Default.Repository = os.Getenv("REGISTRY_ENDPOINT")
	conf.Registry.Default.Username = os.Getenv("REGISTRY_USERNAME")
	conf.Registry.Default.Password = os.Getenv("REGISTRY_PASSWORD")
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
	}
}

func (agent *agentModule) Run(ctx context.Context) error {
	if agent.conf.Provider.Runtime == config.ProcessRuntime {
		err := agent.execPreCmds(ctx)
		if err != nil {
			nlog.Warn(err)
		}
	}
	return commands.RunRootCommand(ctx, agent.conf, agent.clients.KubeClient)
}

func (agent *agentModule) execPreCmds(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", filepath.Join(agent.conf.RootDir, "scripts/deploy/cgroup_pre_detect.sh"))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func (agent *agentModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(60 * time.Second)
	select {
	case <-commands.ReadyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return fmt.Errorf("wait agent ready timeout")
	}
}

func (agent *agentModule) Name() string {
	return "agent"
}

func RunAgentWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := newShutdownHookEntry(2 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "agent",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunAgent(runCtx, cancel, conf, shutdownEntry)
}

func RunAgent(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m := NewAgent(conf)
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
		nlog.Fatalf("Agent wait ready failed: %v", err)
	}
	nlog.Info("Agent is ready")
	return m
}
