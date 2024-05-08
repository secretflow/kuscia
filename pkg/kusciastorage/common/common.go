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

package common

const (
	BeanNameForServer          = "ks-server"
	BeanNameForMysqlRepository = "mysql-repository"
	BeanNameForResourceService = "resource-service"
)

// DomainKindList defines supported domain kind list Which used to validate the request kind.
var DomainKindList = []string{
	"dataobjects",
	"datasources",
	"datatables",
	"domainappimages",
	"domainnodes",
	"pods",
	"configmaps",
	"endpoints",
	"secrets",
	"services",
	"daemonsets",
	"deployments",
	"replicasets",
	"statefulsets",
	"cronjobs",
	"jobs",
	"leases",
}

// ClusterKindList defines supported cluster kind list Which used to validate the request kind.
var ClusterKindList = []string{
	"nodes",
	"appimages",
	"domains",
	"kusciatasks",
}
