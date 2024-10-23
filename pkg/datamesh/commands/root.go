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

package commands

import (
	"context"
	"fmt"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/datamesh/bean"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
)

func Run(ctx context.Context, conf *config.DataMeshConfig, kusciaClient kusciaclientset.Interface) error {
	// new app engine
	appEngine := engine.New(&framework.AppConfig{
		Name:    "DataMesh",
		Usage:   "DataMesh",
		Version: meta.KusciaVersionString(),
	})
	// create informer factory
	conf.KusciaClient = kusciaClient
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(ctx.Done())
	// wait for all caches to sync
	kusciaInformerFactory.WaitForCacheSync(ctx.Done())
	if err := injectBean(conf, appEngine); err != nil {
		return err
	}
	return appEngine.Run(ctx)
}

func injectBean(conf *config.DataMeshConfig, appEngine *engine.Engine) error {
	// inject http server bean
	httpServer := bean.NewHTTPServerBean(conf)
	serverName := httpServer.ServerName()
	err := appEngine.UseBeanWithConfig(serverName, httpServer)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}
	// inject grpc server bean
	grpcServer := bean.NewGrpcServerBean(conf)
	serverName = grpcServer.ServerName()
	err = appEngine.UseBeanWithConfig(serverName, grpcServer)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}
	// inject operator bean
	opServer := bean.NewOperatorBean(conf)
	serverName = opServer.ServerName()
	err = appEngine.UseBeanWithConfig(serverName, opServer)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}
	return nil
}
