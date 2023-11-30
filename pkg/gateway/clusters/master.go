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
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	DomainAPIServer      = "apiserver.master.svc"
	ServiceMasterProxy   = "masterproxy"
	ServiceAPIServer     = "apiserver"
	ServiceKusciaStorage = "kusciastorage"
	ServiceHandshake     = "kuscia-handshake"
	virtualHostHandshake = "handshake-virtual-host"
	ServiceKusciaAPI     = "kusciaapi"
)

func GetMasterClusterName() string {
	return fmt.Sprintf("service-%s", ServiceMasterProxy)
}

func AddMasterClusters(ctx context.Context, namespace string, config *config.MasterConfig) error {
	if !config.Master {
		masterProxyCluster, err := generateDefaultCluster(ServiceMasterProxy, config.MasterProxy)
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
		nlog.Infof("add Master cluster:%s", ServiceMasterProxy)
		waitMasterProxyReady(ctx, config.MasterProxy.Path)
	} else {
		if config.APIServer != nil {
			if err := addMasterCluster(ServiceAPIServer, namespace, config.APIServer, config.APIWhitelist); err != nil {
				return err
			}
		}

		if config.KusciaStorage != nil {
			if err := addMasterCluster(ServiceKusciaStorage, namespace, config.KusciaStorage, nil); err != nil {
				return err
			}
		}

		if config.KusciaAPI != nil {
			if err := addMasterCluster(ServiceKusciaAPI, namespace, config.KusciaAPI, nil); err != nil {
				return err
			}
		}
		addMasterHandshakeRoute(xds.InternalRoute)
		addMasterHandshakeRoute(xds.ExternalRoute)
	}
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

func addMasterServiceVirtualHost(cluster, path, namespace, service string, apiWhitelist []string) error {
	internalVh := generateMasterInternalVirtualHost(cluster, path, service, generateMasterServiceDomains(namespace, service), apiWhitelist)
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

func AddMasterProxyVirtualHost(cluster, path, service, namespace, token string) error {
	internalVh := generateMasterInternalVirtualHost(cluster, path, service, generateMasterProxyDomains(), nil)
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

func generateMasterInternalVirtualHost(cluster, path, service string, domains []string, apiWhitelist []string) *route.VirtualHost {
	var prefixRewrite string
	if len(path) > 0 {
		prefixRewrite = strings.TrimSuffix(path, "/") + "/"
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
	if service == ServiceAPIServer {
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

func getMasterNamespace(path string) (string, error) {
	var namespace string
	handshake := fmt.Sprintf("%s%s", strings.TrimSuffix(path, "/"), "/handshake")
	req, err := http.NewRequest("GET", config.InternalServer+handshake, nil)
	if err != nil {
		return namespace, fmt.Errorf("new http request failed with (%s)", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kuscia-Host", fmt.Sprintf("%s.master.svc", ServiceHandshake))
	req.Header.Set(fmt.Sprintf("%s-Cluster", ServiceHandshake), GetMasterClusterName())
	req.Host = fmt.Sprintf("%s.master.svc", ServiceHandshake)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return namespace, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			nlog.Errorf("close response body error: %v", err)
		}
	}()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return namespace, fmt.Errorf("request %s return error: %v", req.Host, err)
	}

	if res.StatusCode != http.StatusOK {
		return namespace, fmt.Errorf("request %s return error code: %v", req.Host, res.StatusCode)
	}

	kusciaStatus := make(map[string]interface{})
	err = json.Unmarshal(data, &kusciaStatus)
	if err != nil {
		return namespace, fmt.Errorf("request %s return non-json body: %s", req.Host, string(data))
	}

	namespace = fmt.Sprintf("%s", kusciaStatus["namespace"])
	return namespace, nil
}

func waitMasterProxyReady(ctx context.Context, path string) {
	timestick := time.NewTicker(2 * time.Second)
	timeout, timeoutCancel := context.WithTimeout(ctx, time.Second*300)
	defer timeoutCancel()
	for {
		select {
		case <-timestick.C:
			namespace, err := getMasterNamespace(path)
			if err == nil {
				nlog.Infof("Get master gateway namespace: %s", namespace)
				return
			}
			nlog.Infof("get master gateway namespace fail: %v, wait for retry", err)
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

	vh.Domains = append(vh.Domains, fmt.Sprintf("%s.master.svc", ServiceHandshake))

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
