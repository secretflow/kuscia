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
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
)

type KusciaHandler struct {
}

func (handler *KusciaHandler) GenerateInternalRoute(dr *kusciaapisv1alpha1.DomainRoute, dp kusciaapisv1alpha1.DomainPort, token string) []*route.Route {
	clusterName := fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, dr.Spec.Destination, dp.Name)
	requestToAdd := []*core.HeaderValueOption{
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
	}
	isReverseTunnel := utils.IsReverseTunnelTransit(dr.Spec.Transit)
	if isReverseTunnel {
		clusterName = utils.EnvoyClusterName
		requestToAdd = append(requestToAdd, &core.HeaderValueOption{
			Header: &core.HeaderValue{
				Key:   utils.HeaderTransitFlag,
				Value: "true",
			},
			AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}
	action := xds.AddDefaultTimeout(generateDefaultRouteAction(dr, clusterName))
	if isReverseTunnel {
		action.HostRewriteSpecifier = &route.RouteAction_AutoHostRewrite{
			AutoHostRewrite: wrapperspb.Bool(false),
		}
		action.HashPolicy = []*route.RouteAction_HashPolicy{
			{
				PolicySpecifier: &route.RouteAction_HashPolicy_Header_{
					Header: &route.RouteAction_HashPolicy_Header{
						HeaderName: ":authority",
					},
				},
				Terminal: true,
			},
		}
		action.Timeout = &duration.Duration{
			Seconds: 300,
		}
		action.IdleTimeout = &duration.Duration{
			Seconds: 300,
		}
	}
	if len(dp.PathPrefix) > 0 {
		action.PrefixRewrite = strings.TrimSuffix(dp.PathPrefix, "/") + "/"
	}
	httpRoute := &route.Route{
		Name: xds.DefaultRouteName,
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
		},
		Action: &route.Route_Route{
			Route: action,
		},
		RequestHeadersToAdd: requestToAdd,
	}
	return []*route.Route{httpRoute}
}

func (handler *KusciaHandler) UpdateDstCluster(dr *kusciaapisv1alpha1.DomainRoute,
	cluster *envoycluster.Cluster) {
	handshakePath := utils.GetHandshakePathOfEndpoint(dr.Spec.Endpoint)
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
					Path: handshakePath,
					RequestHeadersToAdd: []*core.HeaderValueOption{
						{
							Header: &core.HeaderValue{
								Key:   "Kuscia-Host",
								Value: fmt.Sprintf("%s.%s.svc", utils.ServiceHandshake, dr.Spec.Destination),
							},
						},
						{
							Header: &core.HeaderValue{
								Key:   "Kuscia-Source",
								Value: dr.Spec.Source,
							},
							AppendAction: core.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						},
					},
				},
			},
		},
	}
}
