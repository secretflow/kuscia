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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	virtualHostHandshake = "handshake-virtual-host"
)

func GetMasterClusterName() string {
	return fmt.Sprintf("service-%s", utils.ServiceMasterProxy)
}

func AddMasterProxyClusters(ctx context.Context, namespace string, config *config.MasterConfig) error {
	masterProxyCluster, err := generateDefaultCluster(utils.ServiceMasterProxy, config.MasterProxy)
	if err != nil {
		nlog.Fatalf("Generate masterProxy Cluster fail, %v", err)
		return err
	}

	if err := xds.SetKeepAliveForDstCluster(masterProxyCluster, false); err != nil {
		nlog.Error(err)
		return err
	}
	if err := xds.AddOrUpdateCluster(masterProxyCluster); err != nil {
		nlog.Error(err)
		return err
	}
	nlog.Infof("add Master cluster:%s", utils.ServiceMasterProxy)
	waitMasterProxyReady(ctx, config.MasterProxy.Path, config, namespace)
	return nil
}

func AddMasterClusters(ctx context.Context, namespace string, config *config.MasterConfig) error {
	config.Namespace = namespace
	if config.APIServer != nil {
		if err := addMasterCluster(utils.ServiceAPIServer, namespace, config.APIServer, config.APIWhitelist); err != nil {
			return err
		}
	}

	if config.KusciaStorage != nil {
		if err := addMasterCluster(utils.ServiceKusciaStorage, namespace, config.KusciaStorage, nil); err != nil {
			return err
		}
	}

	if config.KusciaAPI != nil {
		if err := addMasterCluster(utils.ServiceKusciaAPI, namespace, config.KusciaAPI, nil); err != nil {
			return err
		}
	}
	addMasterHandshakeRoute(xds.InternalRoute)
	addMasterHandshakeRoute(xds.ExternalRoute)
	return nil
}

func addMasterCluster(service, namespace string, config *config.ClusterConfig, apiWhitelist []string) error {
	localCluster, err := generateDefaultCluster(service, config)
	if err != nil {
		return fmt.Errorf("generate %s Cluster fail, %v", service, err)
	}

	if err := xds.AddOrUpdateCluster(localCluster); err != nil {
		return err
	}

	if err := addMasterServiceVirtualHost(localCluster.Name, config.Path, namespace, service, apiWhitelist); err != nil {
		return err
	}
	return nil
}

func addMasterServiceVirtualHost(cluster, pathPrefix, namespace, service string, apiWhitelist []string) error {
	internalVh := generateMasterInternalVirtualHost(cluster, pathPrefix, service, generateMasterServiceDomains(namespace, service), apiWhitelist)
	if err := xds.AddOrUpdateVirtualHost(internalVh, xds.InternalRoute); err != nil {
		return err
	}

	externalVh, ok := proto.Clone(internalVh).(*route.VirtualHost)
	if !ok {
		nlog.Fatalf("clone virtual host fail")
	}
	externalVh.Name = fmt.Sprintf("%s-external", cluster)

	return xds.AddOrUpdateVirtualHost(externalVh, xds.ExternalRoute)
}

func AddMasterProxyVirtualHost(cluster, pathPrefix, service, namespace, token string) error {
	internalVh := generateMasterInternalVirtualHost(cluster, pathPrefix, service, generateMasterProxyDomains(), nil)
	internalVh.Routes[0].RequestHeadersToAdd = []*core.HeaderValueOption{
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
				Value: namespace,
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

	return xds.AddOrUpdateVirtualHost(internalVh, xds.InternalRoute)
}

func generateMasterInternalVirtualHost(cluster, pathPrefix, service string, domains []string, apiWhitelist []string) *route.VirtualHost {
	var prefixRewrite string
	if len(pathPrefix) > 0 {
		prefixRewrite = strings.TrimSuffix(pathPrefix, "/") + "/"
	}
	virtualHost := &route.VirtualHost{
		Name:    fmt.Sprintf("%s-internal", cluster),
		Domains: domains,
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
							PrefixRewrite: prefixRewrite,
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: cluster,
							},
							HostRewriteSpecifier: &route.RouteAction_AutoHostRewrite{
								AutoHostRewrite: wrapperspb.Bool(true),
							},
						},
					),
				},
			},
		},
	}
	if service == utils.ServiceAPIServer {
		regex := getMasterAPIWhitelistRegex(apiWhitelist)
		if len(regex) > 0 {
			virtualHost.Routes[0].Match.PathSpecifier = &route.RouteMatch_SafeRegex{
				SafeRegex: &matcherv3.RegexMatcher{
					Regex: regex,
				},
			}
		}
	}
	return virtualHost
}

func generateMasterServiceDomains(namespace, service string) []string {
	return []string{
		fmt.Sprintf("%s.master.svc", service),
		fmt.Sprintf("%s.%s.svc", service, namespace),
	}
}

func generateMasterProxyDomains() []string {
	return []string{
		"*.master.svc",
	}
}

func generateDefaultCluster(name string, config *config.ClusterConfig) (*envoycluster.Cluster, error) {
	cluster := &envoycluster.Cluster{
		Name: fmt.Sprintf("service-%s", name),
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address: config.Host,
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: config.Port,
												},
											},
										},
									},
									Hostname: config.Host,
								},
							},
						},
					},
				},
			},
		},
	}
	if config.TLSCert != nil {
		transportSocket, err := xds.GenerateUpstreamTLSConfigByCert(config.TLSCert)
		if err != nil {
			return nil, err
		}
		cluster.TransportSocket = transportSocket
	}

	if err := xds.DecorateRemoteUpstreamCluster(cluster, config.Protocol); err != nil {
		return nil, err
	}
	return cluster, nil
}

func getMasterNamespace(soure string, pathPrefix string) (string, error) {
	kusciaStatus := map[string]interface{}{}
	handshakePath := utils.GetHandshakePathOfPrefix(pathPrefix)
	err := utils.DoHTTP(nil, &kusciaStatus, &utils.HTTPParam{
		Method:       http.MethodGet,
		Path:         handshakePath,
		KusciaHost:   fmt.Sprintf("%s.master.svc", utils.ServiceHandshake),
		ClusterName:  GetMasterClusterName(),
		KusciaSource: soure,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s", kusciaStatus["namespace"]), nil
}

func waitMasterProxyReady(ctx context.Context, path string, config *config.MasterConfig, namespace string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout, timeoutCancel := context.WithTimeout(ctx, time.Second*300)
	defer timeoutCancel()
	for {
		select {
		case <-ticker.C:
			masterNamespace, err := getMasterNamespace(namespace, path)
			if err == nil {
				nlog.Infof("Get master gateway namespace: %s", masterNamespace)
				config.Namespace = masterNamespace
				return
			}
			nlog.Warnf("get master gateway namespace fail: %v, wait for retry", err)
		case <-timeout.Done():
			nlog.Fatalf("get Master gateway namespace timeout")
		case <-ctx.Done():
			return
		}
	}
}

func addMasterHandshakeRoute(routeName string) {
	vh, err := xds.QueryVirtualHost(virtualHostHandshake, routeName)
	if err != nil {
		nlog.Fatalf("%v", err)
	}

	vh.Domains = append(vh.Domains, fmt.Sprintf("%s.master.svc", utils.ServiceHandshake))

	if err := xds.AddOrUpdateVirtualHost(vh, routeName); err != nil {
		nlog.Fatalf("%v", err)
	}
}

func getMasterAPIWhitelistRegex(apiWhitelist []string) string {
	var result = ""
	if len(apiWhitelist) > 0 {
		result = "(" + strings.Join(apiWhitelist, ")|(") + ")"
	}
	return result
}
