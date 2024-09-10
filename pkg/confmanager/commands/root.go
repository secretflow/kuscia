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

	"github.com/secretflow/kuscia/pkg/confmanager/bean"
	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
)

func Run(ctx context.Context, conf *config.ConfManagerConfig) error {
	// new app engine
	appEngine := engine.New(&framework.AppConfig{
		Name:    "ConfManager",
		Usage:   "ConfManager",
		Version: meta.KusciaVersionString(),
	})
	if err := injectBean(ctx, conf, appEngine); err != nil {
		return err
	}
	return appEngine.Run(ctx)
}

func injectBean(ctx context.Context, conf *config.ConfManagerConfig, appEngine *engine.Engine) error {
	certService := service.NewCertificateService(&service.CertificateServiceConfig{
		DomainCertValue: conf.DomainCertValue,
		DomainKey:       conf.DomainKey,
	})
	configService, err := service.NewConfigService(ctx, &service.ConfigServiceConfig{
		DomainID:   conf.DomainID,
		DomainKey:  conf.DomainKey,
		Driver:     conf.Driver,
		KubeClient: conf.KubeClient,
	})
	if err != nil {
		return err
	}

	// inject http server bean
	httpServer := bean.NewHTTPServerBean(conf, certService, configService)
	serverName := httpServer.ServerName()
	err = appEngine.UseBeanWithConfig(serverName, httpServer)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}
	// inject grpc server bean
	grpcServer := bean.NewGrpcServerBean(conf, certService, configService)
	serverName = grpcServer.ServerName()
	err = appEngine.UseBeanWithConfig(serverName, grpcServer)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}
	return nil
}
