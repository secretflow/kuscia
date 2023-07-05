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

package clusters

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	transportService = "transport"
	schedulerService = "interconn-scheduler"
)

type InterConnClusterConfig struct {
	TransportConfig *ClusterConfig
	SchedulerConfig *ClusterConfig
}

func AddInterConnClusters(namespace string, config *InterConnClusterConfig) error {
	if config.TransportConfig != nil {
		if err := addTransportCluster(namespace, config.TransportConfig); err != nil {
			return err
		}
	}
	if config.SchedulerConfig != nil {
		if err := addSchedulerCluster(namespace, config.SchedulerConfig); err != nil {
			return err
		}
	}
	return nil
}

func addTransportCluster(namespace string, clusterConfig *ClusterConfig) error {
	cluster, err := generateDefaultCluster(transportService, clusterConfig)
	if err != nil {
		return fmt.Errorf("generate %s cluster err: %v", transportService, err)
	}
	if err := xds.AddOrUpdateCluster(cluster); err != nil {
		return err
	}

	internalVh := generateInterConnInternalVirtualHost(transportService, cluster.Name, namespace)
	if err := xds.AddOrUpdateVirtualHost(internalVh, xds.InternalRoute); err != nil {
		return err
	}

	externalVh, ok := proto.Clone(internalVh).(*route.VirtualHost)
	if !ok {
		nlog.Fatalf("Clone virtual host fail")
	}
	externalVh.Name = fmt.Sprintf("%s-external", transportService)
	if err := xds.AddOrUpdateVirtualHost(externalVh, xds.ExternalRoute); err != nil {
		return err
	}
	nlog.Infof("Add Transport Cluster success")
	return nil
}

func addSchedulerCluster(namespace string, clusterConfig *ClusterConfig) error {
	cluster, err := generateDefaultCluster(schedulerService, clusterConfig)
	if err != nil {
		return fmt.Errorf("generate %s cluster err: %v", schedulerService, err)
	}
	if err := xds.AddOrUpdateCluster(cluster); err != nil {
		return err
	}
	externalVh := generateInterConnInternalVirtualHost(schedulerService, cluster.Name, namespace)
	externalVh.Name = fmt.Sprintf("%s-external", schedulerService)
	if err := xds.AddOrUpdateVirtualHost(externalVh, xds.ExternalRoute); err != nil {
		return err
	}
	nlog.Infof("Add interconnection scheduler Cluster success")
	return nil
}

func generateInterConnInternalVirtualHost(service, cluster, namespace string) *route.VirtualHost {
	virtualHost := &route.VirtualHost{
		Name: fmt.Sprintf("%s-internal", service),
		Domains: []string{
			fmt.Sprintf("%s.%s.svc", service, namespace),
			service,
		},
		Routes: []*route.Route{
			{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				Action: &route.Route_Route{
					Route: xds.AddDefaultTimeout(
						&route.RouteAction{
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: cluster,
							},
						},
					),
				},
			},
		},
	}
	return virtualHost
}
