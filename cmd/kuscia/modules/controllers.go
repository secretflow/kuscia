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

//nolint:dupl
package modules

import (
	"context"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/clusterdomainroute"
	"github.com/secretflow/kuscia/pkg/controllers/domain"
	"github.com/secretflow/kuscia/pkg/controllers/domaindata"
	"github.com/secretflow/kuscia/pkg/controllers/domainroute"
	"github.com/secretflow/kuscia/pkg/controllers/garbagecollection"
	"github.com/secretflow/kuscia/pkg/controllers/kusciadeployment"
	"github.com/secretflow/kuscia/pkg/controllers/kusciajob"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask"
	"github.com/secretflow/kuscia/pkg/controllers/portflake"
	"github.com/secretflow/kuscia/pkg/controllers/taskresourcegroup"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewControllersModule(i *ModuleRuntimeConfigs) (Module, error) {
	ctx := context.Background()

	// 初始化 GC ConfigMap
	namespace := "kuscia-system"
	if i.DomainID != "" {
		namespace = i.DomainID
	}

	if err := garbagecollection.InitGCConfigMap(ctx, i.Clients.KubeClient, namespace); err != nil {
		nlog.Warnf("Failed to init GC ConfigMap: %v", err)
	}

	// 创建 GC ConfigManager
	gcConfigManager, err := garbagecollection.NewGCConfigManager(
		i.Clients.KubeClient,
		namespace,
		garbagecollection.DefaultConfigMapName,
	)
	if err != nil {
		nlog.Warnf("Failed to create GC ConfigManager: %v, GC API will not be available", err)
	} else {
		// 启动 ConfigManager
		go func() {
			if err := gcConfigManager.Start(ctx); err != nil {
				nlog.Errorf("Failed to start GC ConfigManager: %v", err)
			}
		}()

		// 存储到模块配置中
		i.GCConfigManager = gcConfigManager
	}

	// Parse garbage collection configuration
	kddGcEnabled := false
	gcDurationHours := 0
	kjGcDurationHours := 0
	// If Enable is nil (not set in config), kddGcEnabled remains false (default value)
	// If Enable is false, kddGcEnabled will be set to false
	// If Enable is true, kddGcEnabled will be set to true
	if i.GarbageCollection.KusciaDomainDataGC.Enable != nil {
		kddGcEnabled = *i.GarbageCollection.KusciaDomainDataGC.Enable
		if kddGcEnabled && i.GarbageCollection.KusciaDomainDataGC.DurationHours > 0 {
			gcDurationHours = i.GarbageCollection.KusciaDomainDataGC.DurationHours
		}
	}
	if i.GarbageCollection.KusciaJobGC.DurationHours > 0 {
		kjGcDurationHours = i.GarbageCollection.KusciaJobGC.DurationHours
	}
	opt := &controllers.Options{
		ControllerName:              "kuscia-controller-manager",
		HealthCheckPort:             8090,
		Workers:                     4,
		RunMode:                     i.RunMode,
		Namespace:                   i.DomainID,
		RootDir:                     i.RootDir,
		EnableWorkloadApprove:       i.EnableWorkloadApprove,
		KddGarbageCollectionEnabled: kddGcEnabled,
		DomainDataGCDurationHours:   gcDurationHours,
		KusciaJobGCDurationHours:    kjGcDurationHours,
		GCConfigManager:             i.GCConfigManager,
	}

	server := controllers.NewServer(
		opt, i.Clients,
		[]controllers.ControllerConstruction{
			{
				NewControler: taskresourcegroup.NewController,
				CRDNames:     []string{controllers.CRDTaskResourcesGroupsName, controllers.CRDTaskResourcesName},
			},
			{
				NewControler: domain.NewController,
				CRDNames:     []string{controllers.CRDDomainsName},
			},
			{
				NewControler: kusciatask.NewController,
				CRDNames:     []string{controllers.CRDKusciaTasksName, controllers.CRDAppImagesName},
			},
			{
				NewControler: domainroute.NewController,
				CRDNames:     []string{controllers.CRDDomainsName, controllers.CRDDomainRoutesName, controllers.CRDGatewaysName},
			},
			{
				NewControler: clusterdomainroute.NewController,
				CRDNames:     []string{controllers.CRDDomainsName, controllers.CRDClusterDomainRoutesName, controllers.CRDDomainRoutesName, controllers.CRDGatewaysName},
			},
			{
				NewControler: kusciajob.NewController,
				CRDNames:     []string{controllers.CRDKusciaJobsName},
			},
			{
				NewControler: kusciadeployment.NewController,
				CRDNames:     []string{controllers.CRDKusciaDeploymentsName},
			},
			{
				NewControler: domaindata.NewController,
				CRDNames:     []string{controllers.CRDDomainsName, controllers.CRDDomainDataGrantsName},
			},
			{
				NewControler: portflake.NewController,
			},

			{
				NewControler: garbagecollection.NewKusciaJobGCController,
			},
			{
				NewControler: garbagecollection.NewKusciaDomainDataGCController,
			},
		},
	)

	// 注意: TriggerManager 会在 KusciaJobGCController 初始化时自动注册到全局变量
	// 在 Server 启动后,KusciaAPI 可以通过 garbagecollection.GetGlobalKusciaJobGCTriggerManager() 获取

	return server, nil
}
