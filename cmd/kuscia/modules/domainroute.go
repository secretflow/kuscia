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
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/gateway/commands"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type domainRouteModule struct {
	conf    *config.GatewayConfig
	clients *kubeconfig.KubeClients
}

func NewDomainRoute(i *Dependencies) Module {
	conf := config.DefaultStaticGatewayConfig()
	conf.RootDir = i.RootDir
	conf.ConfBasedir = filepath.Join(i.RootDir, ConfPrefix, "domainroute")
	conf.Namespace = i.DomainID
	conf.DomainKeyFile = i.DomainKeyFile
	conf.MasterConfig = i.Master
	conf.ExternalTLS = i.ExternalTLS

	if i.TransportPort > 0 {
		conf.TransportConfig = &kusciaconfig.ServiceConfig{
			Endpoint: fmt.Sprintf("http://127.0.0.1:%d", i.TransportPort),
		}
	}
	if i.InterConnSchedulerPort > 0 {
		conf.InterConnSchedulerConfig = &kusciaconfig.ServiceConfig{
			Endpoint: fmt.Sprintf("http://127.0.0.1:%d", i.InterConnSchedulerPort),
		}
	}

	return &domainRouteModule{
		conf:    conf,
		clients: i.Clients,
	}
}

func (d *domainRouteModule) Run(ctx context.Context) error {
	return commands.Run(ctx, d.conf, d.clients)
}

func (d *domainRouteModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(60 * time.Second)
	select {
	case <-commands.ReadyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return fmt.Errorf("wait domainroute ready timeout")
	}
}

func (d *domainRouteModule) Name() string {
	return "domainroute"
}

func RunDomainRoute(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewDomainRoute(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Errorf("domain route wait ready failed with error: %v", err)
		cancel()
	} else {
		nlog.Info("domainroute is ready")
	}

	return m
}
