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
)

func NewControllersModule(i *ModuleRuntimeConfigs) (Module, error) {
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
	}

	return controllers.NewServer(
		opt, i.Clients,
		[]controllers.ControllerConstruction{
			{
				NewController: taskresourcegroup.NewController,
				CRDNames:      []string{controllers.CRDTaskResourcesGroupsName, controllers.CRDTaskResourcesName},
			},
			{
				NewController: domain.NewController,
				CRDNames:      []string{controllers.CRDDomainsName},
			},
			{
				NewController: kusciatask.NewController,
				CRDNames:      []string{controllers.CRDKusciaTasksName, controllers.CRDAppImagesName},
			},
			{
				NewController: domainroute.NewController,
				CRDNames:      []string{controllers.CRDDomainsName, controllers.CRDDomainRoutesName, controllers.CRDGatewaysName},
			},
			{
				NewController: clusterdomainroute.NewController,
				CRDNames:      []string{controllers.CRDDomainsName, controllers.CRDClusterDomainRoutesName, controllers.CRDDomainRoutesName, controllers.CRDGatewaysName},
			},
			{
				NewController: kusciajob.NewController,
				CRDNames:      []string{controllers.CRDKusciaJobsName},
			},
			{
				NewController: kusciadeployment.NewController,
				CRDNames:      []string{controllers.CRDKusciaDeploymentsName},
			},
			{
				NewController: domaindata.NewController,
				CRDNames:      []string{controllers.CRDDomainsName, controllers.CRDDomainDataGrantsName},
			},
			{
				NewController: portflake.NewController,
			},

			{
				NewController: garbagecollection.NewKusciaJobGCController,
			},
			{
				NewController: garbagecollection.NewKusciaDomainDataGCController,
			},
		},
	), nil
}
