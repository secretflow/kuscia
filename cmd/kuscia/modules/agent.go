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
	"path"
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/agent/commands"
	"github.com/secretflow/kuscia/pkg/agent/config"
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
	conf.Namespace = i.DomainID
	hostname, err := os.Hostname()
	if err != nil {
		nlog.Fatalf("Get hostname fail: %v", err)
	}
	conf.StdoutPath = filepath.Join(i.RootDir, StdoutPrefix)
	if conf.Node.NodeName == "" {
		conf.Node.NodeName = hostname
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

	nlog.Debugf("Agent config is %+v", conf)

	return &agentModule{
		conf:    conf,
		clients: i.Clients,
	}
}

func (agent *agentModule) Run(ctx context.Context) error {
	return commands.RunRootCommand(ctx, agent.conf, agent.clients.KubeClient)
}

func (agent *agentModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
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

func RunAgent(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewAgent(conf)
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
		nlog.Info("agent is ready")
	}
	return m
}
