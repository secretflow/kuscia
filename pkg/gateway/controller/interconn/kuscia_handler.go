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

package interconn

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
)

type KusciaHandler struct {
}

func (handler *KusciaHandler) GenerateInternalRoute(dr *kusciaapisv1alpha1.DomainRoute,
	dp kusciaapisv1alpha1.DomainPort, token string) []*route.Route {
	httpRoute := &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
		},
		Action: &route.Route_Route{
			Route: xds.AddDefaultTimeout(generateDefaultRouteAction(dr, dp)),
		},
		RequestHeadersToAdd: []*core.HeaderValueOption{
			{
				Header: &core.HeaderValue{
					Key:   interConnProtocolHeader,
					Value: string(kusciaapisv1alpha1.InterConnKuscia),
				},
				AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
			{
				Header: &core.HeaderValue{
					Key:   "Kuscia-Host",
					Value: "%REQ(:authority)%",
				},
				AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
			{
				Header: &core.HeaderValue{
					Key:   "Kuscia-Source",
					Value: dr.Spec.Source,
				},
				AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
			{
				Header: &core.HeaderValue{
					Key:   "Kuscia-Token",
					Value: token,
				},
				AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
		},
	}
	return []*route.Route{httpRoute}
}

func (handler *KusciaHandler) UpdateDstCluster(dr *kusciaapisv1alpha1.DomainRoute,
	cluster *envoycluster.Cluster) {
	cluster.HealthChecks = []*core.HealthCheck{
		{
			Timeout:            durationpb.New(time.Second),
			Interval:           durationpb.New(15 * time.Second),
			UnhealthyInterval:  durationpb.New(3 * time.Second),
			UnhealthyThreshold: wrapperspb.UInt32(1),
			HealthyThreshold:   wrapperspb.UInt32(1),
			HealthChecker: &core.HealthCheck_HttpHealthCheck_{
				HttpHealthCheck: &core.HealthCheck_HttpHealthCheck{
					Host: dr.Spec.Endpoint.Host,
					Path: "/",
					RequestHeadersToAdd: []*core.HeaderValueOption{
						{
							Header: &core.HeaderValue{
								Key:   "Kuscia-Host",
								Value: fmt.Sprintf("kuscia-handshake.%s.svc", dr.Spec.Destination),
							},
						},
					},
				},
			},
		},
	}
}
