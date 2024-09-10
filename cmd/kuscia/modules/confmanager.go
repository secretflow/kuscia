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
	"sync/atomic"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/commands"
	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
)

const (
	serverCertsCommonName        = "ConfManager"
	defaultServerCertsSanDNSName = "confmanager"
)

type confManagerModule struct {
	moduleRuntimeBase
	conf *config.ConfManagerConfig
}

func NewConfManager(d *ModuleRuntimeConfigs) (Module, error) {
	// overwrite config
	conf := config.NewDefaultConfManagerConfig()
	if d.ConfManager != nil && d.ConfManager.HTTPPort != 0 {
		conf.HTTPPort = d.ConfManager.HTTPPort
	}
	if d.ConfManager != nil && d.ConfManager.GRPCPort != 0 {
		conf.GRPCPort = d.ConfManager.GRPCPort
	}
	if d.ConfManager != nil && d.ConfManager.ConnectTimeout != 0 {
		conf.ConnectTimeout = d.ConfManager.ConnectTimeout
	}
	if d.ConfManager != nil && d.ConfManager.ReadTimeout != 0 {
		conf.ReadTimeout = d.ConfManager.ReadTimeout
	}
	if d.ConfManager != nil && d.ConfManager.WriteTimeout != 0 {
		conf.WriteTimeout = d.ConfManager.WriteTimeout
	}
	if d.ConfManager != nil && d.ConfManager.IdleTimeout != 0 {
		conf.IdleTimeout = d.ConfManager.IdleTimeout
	}
	if d.ConfManager != nil && d.ConfManager.IsMaster {
		conf.IsMaster = d.ConfManager.IsMaster
	}
	if d.ConfManager != nil && d.ConfManager.Driver != "" {
		conf.Driver = d.ConfManager.Driver
	}
	if d.ConfManager != nil && len(d.ConfManager.Params) != 0 {
		conf.Params = d.ConfManager.Params
	}
	conf.DomainID = d.DomainID
	conf.DomainKey = d.DomainKey
	conf.TLS.RootCA = d.CACert
	conf.TLS.RootCAKey = d.CAKey
	conf.KubeClient = d.Clients.KubeClient
	switch d.RunMode {
	case common.RunModeLite:
		conf.DomainCertValue = &d.DomainCertByMasterValue
	case common.RunModeAutonomy:
		conf.DomainCertValue = &atomic.Value{}
		conf.DomainCertValue.Store(d.DomainCert)
	}
	nlog.Infof("Conf manager config is %+v", conf)

	if err := conf.TLS.GenerateServerKeyCerts(serverCertsCommonName, nil, []string{defaultServerCertsSanDNSName}); err != nil {
		return nil, err
	}

	return &confManagerModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "config",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewFuncReadyZ(func(ctx context.Context) error {
				return KusciaServiceReadyZ(&conf.TLS, conf.HTTPPort)
			}),
		},
		conf: conf,
	}, nil
}

func (m *confManagerModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf)
}
