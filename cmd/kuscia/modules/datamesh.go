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
	"time"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/datamesh/commands"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
)

type dataMeshModule struct {
	moduleRuntimeBase
	conf         *config.DataMeshConfig
	kusciaClient kusciaclientset.Interface
}

func NewDataMesh(d *ModuleRuntimeConfigs) (Module, error) {
	conf := config.NewDefaultDataMeshConfig()
	conf.RootDir = d.RootDir
	conf.DomainKey = d.DomainKey

	// override data proxy config
	if d.DataMesh != nil {
		conf.DisableTLS = d.DataMesh.DisableTLS
		conf.DataProxyList = d.DataMesh.DataProxyList
	}

	conf.TLS.RootCA = d.CACert
	conf.TLS.RootCAKey = d.CAKey
	if err := conf.TLS.GenerateServerKeyCerts("DataMesh", nil, []string{"datamesh"}); err != nil {
		return nil, err
	}

	// set log
	interceptorLogger, err := initInterceptorLogger(d.KusciaConfig, dateMeshInterceptorLoggerPath)
	if err != nil {
		nlog.Errorf("Init DataMesh interceptor logger failed: %v", err)
		return nil, err
	}
	conf.InterceptorLog = interceptorLogger
	// set namespace
	conf.KubeNamespace = d.DomainID
	nlog.Infof("Datamesh namespace:%s.", d.DomainID)
	return &dataMeshModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "datamesh",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewFuncReadyZ(func(ctx context.Context) error {
				return KusciaServiceReadyZ(&conf.TLS, conf.HTTPPort)
			}),
		},
		conf:         conf,
		kusciaClient: d.Clients.KusciaClient,
	}, nil
}

func (m *dataMeshModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf, m.kusciaClient)
}
