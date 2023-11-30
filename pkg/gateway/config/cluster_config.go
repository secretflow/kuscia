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

package config

import (
	"github.com/secretflow/kuscia/pkg/gateway/xds"
)

type ClusterConfig struct {
	Host     string
	Port     uint32
	Path     string
	Protocol string

	TLSCert *xds.TLSCert
}

type InterConnClusterConfig struct {
	TransportConfig *ClusterConfig
	SchedulerConfig *ClusterConfig
}

type MasterConfig struct {
	Master        bool
	MasterProxy   *ClusterConfig
	APIServer     *ClusterConfig
	KusciaStorage *ClusterConfig
	KusciaAPI     *ClusterConfig
	APIWhitelist  []string
}
