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

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

const (
	interConnProtocolHeader = "x-interconn-protocol"
)

var Decorator DomainRouteDecorator

func init() {
	Decorator = NewDomainRouteDecorator()
}

type DomainRouteDecorator interface {
	GenerateInternalRoute(dr *kusciaapisv1alpha1.DomainRoute, dp kusciaapisv1alpha1.DomainPort,
		token string) []*route.Route
	UpdateDstCluster(dr *kusciaapisv1alpha1.DomainRoute, cluster *envoycluster.Cluster)
}

type Factory struct {
	handlers map[kusciaapisv1alpha1.InterConnProtocolType]DomainRouteDecorator
}

func NewDomainRouteDecorator() DomainRouteDecorator {
	factory := &Factory{
		handlers: map[kusciaapisv1alpha1.InterConnProtocolType]DomainRouteDecorator{
			kusciaapisv1alpha1.InterConnKuscia: &KusciaHandler{},
			kusciaapisv1alpha1.InterConnBFIA:   &BFIAHandler{},
		},
	}
	return factory
}

func (f *Factory) GenerateInternalRoute(dr *kusciaapisv1alpha1.DomainRoute, dp kusciaapisv1alpha1.DomainPort,
	token string) []*route.Route {
	interConnProtocol := getInterConnProtocol(dr)
	return f.handlers[interConnProtocol].GenerateInternalRoute(dr, dp, token)
}

func (f *Factory) UpdateDstCluster(dr *kusciaapisv1alpha1.DomainRoute,
	cluster *envoycluster.Cluster) {
	interConnProtocol := getInterConnProtocol(dr)
	f.handlers[interConnProtocol].UpdateDstCluster(dr, cluster)
}

func generateDefaultRouteAction(dr *kusciaapisv1alpha1.DomainRoute,
	dp kusciaapisv1alpha1.DomainPort) *route.RouteAction {
	action := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: fmt.Sprintf("%s-to-%s-%s", dr.Spec.Source, dr.Spec.Destination, dp.Name),
		},
		HostRewriteSpecifier: &route.RouteAction_AutoHostRewrite{
			AutoHostRewrite: wrapperspb.Bool(true),
		},
	}
	return action
}

func getInterConnProtocol(dr *kusciaapisv1alpha1.DomainRoute) kusciaapisv1alpha1.InterConnProtocolType {
	if len(dr.Spec.InterConnProtocol) > 0 {
		return dr.Spec.InterConnProtocol
	}
	return kusciaapisv1alpha1.InterConnKuscia
}
