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

package controllers

import (
	"context"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

type IController interface {
	Run(int) error
	Stop()
	Name() string
}

type ControllerConstruction struct {
	NewControler NewControllerFunc
	CheckCRD     CheckCRDExistsFunc
}

type NewControllerFunc func(ctx context.Context, config ControllerConfig) IController
type CheckCRDExistsFunc func(ctx context.Context, extensionClient apiextensionsclientset.Interface) error

const (
	CRDAppImagesName           = "appimages.kuscia.secretflow"
	CRDClusterDomainRoutesName = "clusterdomainroutes.kuscia.secretflow"
	CRDDataObjectsName         = "dataobjects.kuscia.secretflow"
	CRDDataResourcesName       = "datasources.kuscia.secretflow"
	CRDDataTablesName          = "datatables.kuscia.secretflow"
	CRDDomainAppImagesName     = "domainappimages.kuscia.secretflow"
	CRDDomainRoutesName        = "domainroutes.kuscia.secretflow"
	CRDDomainsName             = "domains.kuscia.secretflow"
	CRDGatewaysName            = "gateways.kuscia.secretflow"
	CRDKusciaTasksName         = "kusciatasks.kuscia.secretflow"
	CRDKusciaDeploymentsName   = "kusciadeployments.kuscia.secretflow"
	CRDTaskResourcesGroupsName = "taskresourcegroups.kuscia.secretflow"
	CRDTaskResourcesName       = "taskresources.kuscia.secretflow"
	CRDKusciaJobsName          = "kusciajobs.kuscia.secretflow"
)

type ControllerConfig struct {
	IsMaster      bool
	Namespace     string
	RootDir       string
	KubeClient    kubernetes.Interface
	KusciaClient  kusciaclientset.Interface
	EventRecorder record.EventRecorder
}
