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

//nolint:dulp
package modules

import (
	"context"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/clusterdomainroute"
	"github.com/secretflow/kuscia/pkg/controllers/domain"
	"github.com/secretflow/kuscia/pkg/controllers/domaindata"
	"github.com/secretflow/kuscia/pkg/controllers/kusciadeployment"
	"github.com/secretflow/kuscia/pkg/controllers/kusciajob"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask"
	"github.com/secretflow/kuscia/pkg/controllers/taskresourcegroup"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewControllersModule(i *Dependencies) Module {
	opt := &controllers.Options{
		ControllerName:  "kuscia-controller-manager",
		HealthCheckPort: 8090,
		Workers:         4,
		IsMaster:        i.IsMaster,
		Namespace:       i.DomainID,
		RootDir:         i.RootDir,
	}

	return controllers.NewServer(
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
		},
	)
}

func RunController(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewControllersModule(conf)
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
		nlog.Info("controllers is ready")
	}

	return m
}
