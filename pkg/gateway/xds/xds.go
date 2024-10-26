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

//nolint:all
package xds

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	// envoy build-in plugins for Unmarshal listeners
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	bandwidth_limitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/bandwidth_limit/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_bridge/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_reverse_bridge/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	_ "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_gress/v3"

	// kuscia extend plugins for Unmarshal listeners
	headerdecorator "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_header_decorator/v3"
	kusciatoken "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_token_auth/v3"

	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	grpcMaxConcurrentStreams = 1000000
	InternalRoute            = "internal-route"
	ExternalRoute            = "external-route"
	DefaultVirtualHost       = "default-virtual-host"
	ExternalListener         = "external-listener"
	InternalListener         = "internal-listener"
	InternalTLSPort          = 443
	DefaultRouteName         = "default"
)

const (
	tokenAuthFilterName       = "envoy.filters.http.kuscia_token_auth"
	cryptFilterName           = "envoy.filters.http.kuscia_crypt"
	headerDecoratorFilterName = "envoy.filters.http.kuscia_header_decorator"
	receiverFilterName        = "envoy.filters.http.kuscia_receiver"
	pollerFilterName          = "envoy.filters.http.kuscia_poller"
)

var (
	// IdleTimeout bounds the amount of time the request’s stream may be idle.
	// After header decoding, the idle timeout will apply on downstream and upstream request events.
	// Each time an encode/decode event for headers or data is processed for the stream, the timer will be reset.
	// If the timeout fires, the stream is terminated with a 408 Request Timeout error code if no upstream response header has been received, otherwise a stream reset occurs.
	IdleTimeout int
)

var (
	snapshotCache cache.SnapshotCache
	nodeID        string
	snapshot      *cache.Snapshot
	lock          sync.Mutex
	ctx           context.Context
	config        *InitConfig
)

type InitConfig struct {
	Basedir      string
	Logdir       string
	XDSPort      uint32
	ExternalPort uint32

	ExternalCert *TLSCert
	InternalCert *TLSCert
}

type ConfigTemplate struct {
	Namespace    string
	Instance     string
	ExternalPort uint32
	LogPrefix    string
	Version      string // kuscia version
}

type RouteLimitConfig struct {
	Services  []string
	LimitKbps int64
}

func NewXdsServer(port uint32, id string) {
	// Create a cache
	snapshotCache = cache.NewSnapshotCache(false, cache.IDHash{}, nil)
	nodeID = id
	virtualHostLimits = map[string]map[string]*RouteLimitConfig{}

	// Run the xDS server
	ctx = context.Background()
	cb := &test.Callbacks{Debug: false}
	srv := server.NewServer(ctx, snapshotCache, cb)
	go runServer(ctx, srv, port)
}

// RunServer starts an xDS server at the given port.
func runServer(ctx context.Context, srv server.Server, port uint32) {
	// gRPC golang library sets a very small upper bound for the number gRPC/h2
	// streams over a single TCP connection. If a proxy multiplexes requests over
	// a single connection to the management server, then it might lead to
	// availability problems.
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)

	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		nlog.Fatal(err)
	}

	registerServer(grpcServer, srv)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		nlog.Infof("Management server listening on %d", port)
		if err = grpcServer.Serve(lis); err != nil {
			nlog.Fatal(err)
		}
	}()

	select {
	case <-ctx.Done():
		// notity server stop
		grpcServer.Stop()
	case <-ch:
	}
}

func registerServer(grpcServer *grpc.Server, server server.Server) {
	// register services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, server)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, server)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, server)
}

func InitSnapshot(ns, instance string, initConfig *InitConfig) {
	config = initConfig
	// Add the snapshot to the cache
	var err error
	configTemplate := ConfigTemplate{
		Namespace:    ns,
		Instance:     instance,
		ExternalPort: config.ExternalPort,
		LogPrefix:    config.Logdir,
		Version:      meta.KusciaVersionString(),
	}
	snapshot, err = cache.NewSnapshot("1", map[resource.Type][]types.Resource{
		resource.ClusterType:  generateClusters(config.Basedir),
		resource.RouteType:    generateRoutes(configTemplate, config.Basedir),
		resource.ListenerType: generateListeners(configTemplate, config),
	})
	if err != nil {
		nlog.Fatalf("new snapshot failed with %v", err)
	}
	if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		nlog.Fatalf("init snapshot failed with %v", err)
	}
}

func generateListeners(configTemplate ConfigTemplate, config *InitConfig) []types.Resource {
	listenerConfPath := path.Join(config.Basedir, "listeners")
	dir, err := os.ReadDir(listenerConfPath)
	if err != nil {
		nlog.Fatalf("read listener conf path fail with %v", err)
	}

	var buffers []*bytes.Buffer
	for _, name := range dir {
		if !strings.HasSuffix(name.Name(), ".json") { // ignore *.tmpl
			continue
		}
		fileName := path.Join(listenerConfPath, name.Name())
		data, err := os.ReadFile(fileName)
		if err != nil {
			nlog.Fatalf("read file %s failed with %v", fileName, err)
		}
		buffers = append(buffers, bytes.NewBuffer(data))
	}

	externalListenerTmplPath := path.Join(listenerConfPath, "external_listeners.json.tmpl")
	externalListenerTmpl, err := template.ParseFiles(externalListenerTmplPath)
	if err != nil {
		nlog.Fatal(err)
	}
	var externalListener bytes.Buffer
	if err := externalListenerTmpl.Execute(&externalListener, configTemplate); err != nil {
		nlog.Fatal(err)
	}
	buffers = append(buffers, &externalListener)

	internalListenerTmplPath := path.Join(listenerConfPath, "internal_listeners.json.tmpl")
	internalListenerTmpl, err := template.ParseFiles(internalListenerTmplPath)
	if err != nil {
		nlog.Fatal(err)
	}
	var internalListener bytes.Buffer
	if err := internalListenerTmpl.Execute(&internalListener, configTemplate); err != nil {
		nlog.Fatal(err)
	}
	buffers = append(buffers, &internalListener)

	var listeners []types.Resource
	for _, data := range buffers {
		var lis listener.Listener
		if err := protojson.Unmarshal(data.Bytes(), &lis); err != nil {
			nlog.Fatal(err)
		}
		if lis.Name == ExternalListener && config.ExternalCert != nil {
			generateTLSListener(&lis, config.ExternalCert)
		}
		if lis.Name == InternalListener && config.InternalCert != nil {
			tlsLis, err := copyTLSListener(&lis, config.InternalCert, InternalTLSPort)
			if err != nil {
				nlog.Fatalf("clone internal-listener fail, detail: %v", err)
			}
			listeners = append(listeners, tlsLis)
		}

		listeners = append(listeners, &lis)
	}

	return listeners
}

func generateTLSListener(lis *listener.Listener, cert *TLSCert) {
	if cert != nil {
		transportSocket, err := GenerateDownstreamTLSConfigByCert(cert)
		if err != nil {
			nlog.Fatalf("generate tls config failed with %v", err)
		}
		lis.FilterChains[0].TransportSocket = transportSocket
	}
}

func copyTLSListener(lis *listener.Listener, cert *TLSCert, port uint32) (*listener.Listener, error) {
	tlsLis, ok := proto.Clone(lis).(*listener.Listener)
	if !ok {
		return nil, fmt.Errorf("clone %s fail", lis.Name)
	}

	tlsLis.Name = fmt.Sprintf("%s-tls", lis.Name)
	tlsLis.GetAddress().GetSocketAddress().PortSpecifier = &core.SocketAddress_PortValue{
		PortValue: port,
	}

	generateTLSListener(tlsLis, cert)
	return tlsLis, nil
}

func generateRoutes(configTemplate ConfigTemplate, basedir string) []types.Resource {
	routeConfPath := path.Join(basedir, "routes")
	dir, err := os.ReadDir(routeConfPath)
	if err != nil {
		nlog.Fatalf("read route conf path fail with %v", err)
	}

	var buffers []*bytes.Buffer
	for _, name := range dir {
		if !strings.HasSuffix(name.Name(), ".json") { // ignore *.tmpl
			continue
		}
		fileName := path.Join(routeConfPath, name.Name())
		data, err := os.ReadFile(fileName)
		if err != nil {
			nlog.Fatalf("read file %s failed with %v", name.Name(), err)
		}
		buffers = append(buffers, bytes.NewBuffer(data))
	}

	externalRouteTmplPath := path.Join(routeConfPath, "external_route.json.tmpl")
	internalRouteTmplPath := path.Join(routeConfPath, "internal_route.json.tmpl")
	buffers = append(buffers, instantiateTmpRoute(configTemplate, externalRouteTmplPath))
	buffers = append(buffers, instantiateTmpRoute(configTemplate, internalRouteTmplPath))

	var routes []types.Resource
	for _, data := range buffers {
		var routeConfig route.RouteConfiguration
		if err := protojson.Unmarshal(data.Bytes(), &routeConfig); err != nil {
			nlog.Fatal(err)
		}

		routes = append(routes, &routeConfig)
	}

	generateExtraRouteRules(routes, basedir)
	return routes
}

func instantiateTmpRoute(configTemplate ConfigTemplate, tmpPath string) *bytes.Buffer {
	routeTmpl, err := template.ParseFiles(tmpPath)
	if err != nil {
		nlog.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := routeTmpl.Execute(&buffer, configTemplate); err != nil {
		nlog.Fatal(err)
	}
	return &buffer
}

func generateExtraRouteRules(routes []types.Resource, basedir string) {
	routeConfPath := path.Join(basedir, "route_rules")
	dir, err := os.ReadDir(routeConfPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		// extra rules aren't necessary
		nlog.Errorf("Read route_rules conf path fail with %v", err)
		return
	}

	for _, name := range dir {
		fileName := path.Join(routeConfPath, name.Name())
		data, err := os.ReadFile(fileName)
		if err != nil {
			nlog.Error(err)
			continue
		}
		var routeRule route.Route
		if err := protojson.Unmarshal(data, &routeRule); err != nil {
			nlog.Error(err)
			continue
		}

		chunks := strings.Split(routeRule.Name, "_")
		if len(chunks) != 3 {
			nlog.Errorf("invalid name (%s)", routeRule.Name)
			continue
		}

		for _, routeConf := range routes {
			routeConf, ok := routeConf.(*route.RouteConfiguration)
			if !ok {
				nlog.Errorf("extra route_rules cannot cast to RouteConfiguration")
				continue
			}

			if routeConf.Name != chunks[0] {
				continue
			}

			for _, vh := range routeConf.VirtualHosts {
				if vh.Name != chunks[1] {
					continue
				}
				vh.Routes = append([]*route.Route{&routeRule}, vh.Routes...)
			}
		}
	}
}

func generateClusters(basedir string) []types.Resource {
	clusterConfPath := path.Join(basedir, "clusters")
	dir, err := os.ReadDir(clusterConfPath)
	if os.IsNotExist(err) {
		return []types.Resource{}
	}
	if err != nil {
		// clusters aren't necessary
		nlog.Errorf("read cluster conf path fail with %v", err)
		return []types.Resource{}
	}

	var clusters []types.Resource
	for _, name := range dir {
		fileName := path.Join(clusterConfPath, name.Name())
		data, err := os.ReadFile(fileName)
		if err != nil {
			nlog.Errorf("read file %s failed with %v", fileName, err)
			continue
		}
		var cluster envoycluster.Cluster
		if err := protojson.Unmarshal(data, &cluster); err != nil {
			nlog.Error(err)
			continue
		}
		clusters = append(clusters, &cluster)
	}
	return clusters
}

func AddOrUpdateCluster(conf *envoycluster.Cluster) error {
	lock.Lock()
	defer lock.Unlock()
	clusters := snapshot.Resources[types.Cluster].Items
	items := make(map[string]types.ResourceWithTTL)
	for k, v := range clusters {
		if k == conf.Name {
			continue
		}
		items[k] = v
	}
	items[conf.Name] = types.ResourceWithTTL{Resource: conf}

	if err := resetSnapshot(types.Cluster, items); err != nil {
		return err
	}

	nlog.Infof("Add cluster:%s", conf.Name)
	return nil
}

func DeleteCluster(name string) error {
	lock.Lock()
	defer lock.Unlock()
	clusters := snapshot.Resources[types.Cluster].Items
	if len(clusters) == 0 {
		return nil
	}

	if _, ok := clusters[name]; !ok {
		return nil
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range clusters {
		if k == name {
			continue
		}
		items[k] = v
	}

	if err := resetSnapshot(types.Cluster, items); err != nil {
		return err
	}

	nlog.Debugf("Delete cluster:%s", name)
	return nil
}

func QueryCluster(name string) (*envoycluster.Cluster, error) {
	lock.Lock()
	defer lock.Unlock()
	clusters := snapshot.Resources[types.Cluster].Items
	rs, ok := clusters[name]
	if !ok {
		return nil, fmt.Errorf("unknown cluster: %s", name)
	}
	cluster, ok := rs.Resource.(*envoycluster.Cluster)
	if !ok {
		return nil, fmt.Errorf("resource cannot cast to Cluster")
	}

	copiedCluster, ok := proto.Clone(cluster).(*envoycluster.Cluster)
	if !ok {
		return nil, fmt.Errorf("clone cluster (%s) fail", name)
	}
	return copiedCluster, nil
}

func QueryVirtualHost(name, routeName string) (*route.VirtualHost, error) {
	lock.Lock()
	defer lock.Unlock()
	routes := snapshot.Resources[types.Route].Items
	rs, ok := routes[routeName]
	if !ok {
		nlog.Errorf("unknown route config name: %s", routeName)
		return nil, fmt.Errorf("unknown route config name: %s", routeName)
	}

	routeConfig, ok := rs.Resource.(*route.RouteConfiguration)
	if !ok {
		return nil, fmt.Errorf("resource cannot cast to RouteConfiguration")
	}

	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == name {
			return routeConfig.VirtualHosts[i], nil
		}
	}
	return nil, fmt.Errorf("cannot find virtual host (%s) in route (%s)", name, routeName)
}

// svc name is combined with two segments, prefix and port name,
// in the form of ${prefix}-${portname}.
// svc from same task has same prefix, but different port name.
// for example, secretflow-task-20240619143203-single-psi-0-spu is combined with
// prefix secretflow-task-20240619143203-single-psi-0 and port name spu.
// this function can merge prefix of svc names, then generate a much simpler
// match expression than simply expand regex.
// secretflow-task-20240619143203-single-psi-0-spu +
// secretflow-task-20240619143203-single-psi-0-fed +
// secretflow-task-20240619143203-single-psi-0-global
// -> secretflow-task-20240619143203-single-psi-0
func dedupServicePrefix(svcNames []string) []string {
	m := make(map[string]bool)
	for _, svcName := range svcNames {
		if idx := strings.LastIndex(svcName, "-"); idx > 0 {
			m[svcName[:idx]] = true
		}
	}
	var res []string
	for prefix := range m {
		res = append(res, prefix)
	}
	return res
}

func generateMatchExpr(svcNames []string) string {
	var expr string
	for i, svc := range dedupServicePrefix(svcNames) {
		if i == 0 {
			expr = fmt.Sprintf("%s.*", svc)
		} else {
			expr += fmt.Sprintf("|%s.*", svc)
		}
	}
	return expr
}

func updateVhLimitRoute(vh *route.VirtualHost, tasks map[string]*RouteLimitConfig) {
	var defaultRoute *route.Route
	var routes []*route.Route
	for _, route := range vh.Routes {
		// remove older task limit config
		if !strings.HasSuffix(route.Name, "bandwidth-limit") {
			routes = append(routes, route)
			// find default route
			if route.Name == DefaultRouteName {
				defaultRoute = route
			}
		}
	}
	vh.Routes = routes
	if len(tasks) == 0 || defaultRoute == nil {
		return
	}
	// in reverse tunnel cluster mode, envoy uses traffic redirection and consistent
	// hashing to ensure that the traffic is routed to the intended node.
	// This approach, however, results in duplicate traffic, envoy need to exclude
	// redirected traffic in bandwidth limit case.
	var isReverseTunnel bool
	if defaultRoute.GetRoute() != nil && defaultRoute.GetRoute().GetClusterSpecifier() != nil {
		if defaultRoute.GetRoute().GetCluster() == utils.EnvoyClusterName {
			isReverseTunnel = true
		}
	}
	// add or update routes
	for task, cfg := range tasks {
		if cfg.LimitKbps == 0 {
			continue
		}
		r := proto.Clone(defaultRoute).(*route.Route)
		r.Name = fmt.Sprintf("%s-bandwidth-limit", task)
		r.Match.Headers = []*route.HeaderMatcher{
			{
				Name: ":authority", // match host
				HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
					StringMatch: &matcherv3.StringMatcher{
						MatchPattern: &matcherv3.StringMatcher_SafeRegex{
							SafeRegex: &matcherv3.RegexMatcher{
								Regex: generateMatchExpr(cfg.Services),
							},
						},
					},
				},
			},
		}
		if isReverseTunnel {
			r.Match.Headers = append(r.Match.Headers, &route.HeaderMatcher{
				Name:                 utils.HeaderTransitFlag,
				HeaderMatchSpecifier: &route.HeaderMatcher_PresentMatch{PresentMatch: false},
			})
		}
		bandwidthConfig, _ := anypb.New(&bandwidth_limitv3.BandwidthLimit{
			StatPrefix:   "kuscia_bandwidth_limit",
			FillInterval: &durationpb.Duration{Nanos: 1e8}, // 0.1s
			EnableMode:   bandwidth_limitv3.BandwidthLimit_REQUEST_AND_RESPONSE,
			LimitKbps:    &wrapperspb.UInt64Value{Value: uint64(cfg.LimitKbps)},
		})
		r.TypedPerFilterConfig["envoy.filters.http.bandwidth_limit"] = bandwidthConfig
		for i, route := range vh.Routes {
			if route.Name == task {
				vh.Routes[i] = r
				return
			}
		}
		vh.Routes = append([]*route.Route{r}, vh.Routes...)
	}
}

func AddOrUpdateVirtualHost(vh *route.VirtualHost, routeName string) error {
	lock.Lock()
	defer lock.Unlock()
	routes := snapshot.Resources[types.Route].Items
	_, ok := routes[routeName]
	if !ok {
		nlog.Errorf("Unknown route config name: %s", routeName)
		return fmt.Errorf("unknown route config name: %s", routeName)
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range routes {
		if k == routeName {
			res := proto.Clone(routes[k].Resource).(*route.RouteConfiguration)
			items[k] = types.ResourceWithTTL{Resource: res}
		} else {
			items[k] = v
		}
	}

	routeConfig, ok := items[routeName].Resource.(*route.RouteConfiguration)
	if !ok {
		return fmt.Errorf("resource cannot cast to RouteConfiguration")
	}
	// internal route only
	if routeName == InternalRoute {
		updateVhLimitRoute(vh, virtualHostLimits[vh.Name])
	}

	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == vh.Name {
			routeConfig.VirtualHosts = append(routeConfig.VirtualHosts[:i], routeConfig.VirtualHosts[i+1:]...)
			break
		}
	}
	routeConfig.VirtualHosts = append([]*route.VirtualHost{vh}, routeConfig.VirtualHosts...)

	if err := resetSnapshot(types.Route, items); err != nil {
		return err
	}
	return nil
}

func UpdateVirtualHostByName(vhName string, routeName string) error {
	lock.Lock()
	defer lock.Unlock()
	routes := snapshot.Resources[types.Route].Items
	_, ok := routes[routeName]
	if !ok {
		nlog.Errorf("Unknown route config name: %s", routeName)
		return fmt.Errorf("unknown route config name: %s", routeName)
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range routes {
		if k == routeName {
			res := proto.Clone(routes[k].Resource).(*route.RouteConfiguration)
			items[k] = types.ResourceWithTTL{Resource: res}
		} else {
			items[k] = v
		}
	}

	routeConfig, ok := items[routeName].Resource.(*route.RouteConfiguration)
	if !ok {
		return fmt.Errorf("resource cannot cast to RouteConfiguration")
	}

	var vh *route.VirtualHost
	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == vhName {
			vh = routeConfig.VirtualHosts[i]
			break
		}
	}

	if vh == nil {
		return fmt.Errorf("cannot find virtual host (%s) in route (%s)", vhName, routeName)
	}
	// internal route only
	if routeName == InternalRoute {
		updateVhLimitRoute(vh, virtualHostLimits[vh.Name])
	}

	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == vhName {
			routeConfig.VirtualHosts = append(routeConfig.VirtualHosts[:i], routeConfig.VirtualHosts[i+1:]...)
			break
		}
	}
	routeConfig.VirtualHosts = append([]*route.VirtualHost{vh}, routeConfig.VirtualHosts...)

	if err := resetSnapshot(types.Route, items); err != nil {
		return err
	}
	return nil
}

func DeleteVirtualHost(name, routeName string) error {
	lock.Lock()
	defer lock.Unlock()
	routes := snapshot.Resources[types.Route].Items
	_, ok := routes[routeName]
	if !ok {
		nlog.Errorf("unknown route config name: %s", routeName)
		return fmt.Errorf("unknown route config name: %s", routeName)
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range routes {
		if k == routeName {
			res := proto.Clone(routes[k].Resource).(*route.RouteConfiguration)
			items[k] = types.ResourceWithTTL{Resource: res}
		} else {
			items[k] = v
		}
	}

	routeConfig, ok := items[routeName].Resource.(*route.RouteConfiguration)
	if !ok {
		return fmt.Errorf("resource cannot cast to RouteConfiguration")
	}

	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == name {
			routeConfig.VirtualHosts = append(routeConfig.VirtualHosts[:i], routeConfig.VirtualHosts[i+1:]...)
			break
		}
	}

	if err := resetSnapshot(types.Route, items); err != nil {
		return err
	}
	return nil
}

func DeleteRoute(name, vhName, routeName string) error {
	lock.Lock()
	defer lock.Unlock()
	routes := snapshot.Resources[types.Route].Items
	_, ok := routes[routeName]
	if !ok {
		return fmt.Errorf("unknown route config name: %s", routeName)
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range routes {
		if k == routeName {
			res := proto.Clone(routes[k].Resource).(*route.RouteConfiguration)
			items[k] = types.ResourceWithTTL{Resource: res}
		} else {
			items[k] = v
		}
	}
	routeConfig, ok := items[routeName].Resource.(*route.RouteConfiguration)
	if !ok {
		return fmt.Errorf("resource cannot cast to RouteConfiguration")
	}

	var vh *route.VirtualHost
	for i := range routeConfig.VirtualHosts {
		if routeConfig.VirtualHosts[i].Name == vhName {
			vh = routeConfig.VirtualHosts[i]
			break
		}
	}
	if vh == nil {
		return fmt.Errorf("cannot find virtual host (%s) in route (%s)", vhName, routeName)
	}

	for i := range vh.Routes {
		if vh.Routes[i].Name == name {
			vh.Routes = append(vh.Routes[:i], vh.Routes[i+1:]...)
			break
		}
	}

	if err := resetSnapshot(types.Route, items); err != nil {
		return err
	}
	return nil
}

func GetHTTPFilterConfig(filterName, listenerName string) (*anypb.Any, error) {
	lock.Lock()
	defer lock.Unlock()
	return getHTTPFilterConfig(filterName, listenerName)
}

func getHTTPFilterConfig(filterName, listenerName string) (*anypb.Any, error) {
	filterNames := []string{
		filterName,
	}

	configs, err := getHTTPFilterConfigs(filterNames, listenerName)
	if err != nil {
		return nil, err
	}

	if len(configs) != 1 {
		return nil, fmt.Errorf("invalid config size(%d) of %s", len(configs), filterName)
	}
	return configs[0], nil
}

func getHTTPFilterConfigs(filterNames []string, listenerName string) ([]*anypb.Any, error) {
	listeners := snapshot.Resources[types.Listener].Items

	rs, ok := listeners[listenerName]
	if !ok {
		return nil, fmt.Errorf("unknown listener name: %s", listenerName)
	}
	lis, ok := rs.Resource.(*listener.Listener)
	if !ok {
		return nil, fmt.Errorf("resource cannot cast to listener")
	}

	var httpManager hcm.HttpConnectionManager
	if err := lis.FilterChains[0].Filters[0].GetTypedConfig().UnmarshalTo(&httpManager); err != nil {
		// we only have one filter chain contained by one network filter hcm
		return nil, fmt.Errorf("unmarshal hcm failed with %s", err.Error())
	}

	var filters []*anypb.Any
	for _, filterName := range filterNames {
		found := false
		for _, httpFilter := range httpManager.HttpFilters {
			if httpFilter.Name == filterName {
				filters = append(filters, httpFilter.GetTypedConfig())
				found = true
				break
			}
		}
		if !found {
			return filters, fmt.Errorf("no config for %s found in %s", filterName, listenerName)
		}
	}

	return filters, nil
}

func GetHeaderDecorator() (*headerdecorator.HeaderDecorator, error) {
	lock.Lock()
	defer lock.Unlock()

	protoConfig, err := getHTTPFilterConfig(headerDecoratorFilterName, ExternalListener)
	if err != nil {
		return nil, err
	}
	var headerDecoratorFilter headerdecorator.HeaderDecorator
	if err := protoConfig.UnmarshalTo(&headerDecoratorFilter); err != nil {
		return nil, fmt.Errorf("unmarshal kuscia filter failed with %s", err.Error())
	}
	return &headerDecoratorFilter, nil
}

func GetTokenAuth() (*kusciatoken.TokenAuth, error) {
	lock.Lock()
	defer lock.Unlock()

	protoConfig, err := getHTTPFilterConfig(tokenAuthFilterName, ExternalListener)
	if err != nil {
		return nil, err
	}
	var tokenAuthFilter kusciatoken.TokenAuth
	if err := protoConfig.UnmarshalTo(&tokenAuthFilter); err != nil {
		return nil, fmt.Errorf("unmarshal kuscia filter failed with %s", err.Error())
	}
	return &tokenAuthFilter, nil
}

func updateHTTPFilters(filterMap map[string]protoreflect.ProtoMessage, listenerName string) error {
	listeners := snapshot.Resources[types.Listener].Items

	_, ok := listeners[listenerName]
	if !ok {
		return fmt.Errorf("unknown listener name: %s", listenerName)
	}

	items := make(map[string]types.ResourceWithTTL)
	for k, v := range listeners {
		if k == listenerName {
			res := proto.Clone(listeners[k].Resource).(*listener.Listener)
			items[k] = types.ResourceWithTTL{Resource: res}
		} else {
			items[k] = v
		}
	}

	lis, ok := items[listenerName].Resource.(*listener.Listener)
	if !ok {
		return fmt.Errorf("resource cannot cast to listener")
	}

	var httpManager hcm.HttpConnectionManager
	if err := lis.FilterChains[0].Filters[0].GetTypedConfig().UnmarshalTo(&httpManager); err != nil {
		// we only have one filter chain contained by one network filter hcm
		return fmt.Errorf("unmarshal hcm failed with %s", err.Error())
	}

	var filters []*hcm.HttpFilter
	for _, filter := range httpManager.HttpFilters {
		if _, ok := mutableFilters[filter.Name]; !ok {
			filters = append(filters, filter)
		}
	}
	for name, filter := range filterMap {
		typedConfig, _ := anypb.New(filter)
		filters = append(filters, &hcm.HttpFilter{
			Name:       name,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: typedConfig},
		})
	}

	httpManager.HttpFilters = filters
	if listenerName == InternalListener {
		httpManager.HttpFilters = sortInternalFilters(httpManager.HttpFilters)
	} else if listenerName == ExternalListener {
		httpManager.HttpFilters = sortExternalFilters(httpManager.HttpFilters)
	} else {
		return fmt.Errorf("invalid listener: %s", listenerName)
	}

	hcmConfig, err := anypb.New(&httpManager)
	if err != nil {
		return fmt.Errorf("marshal http connection manager failed with %s", err.Error())
	}

	hcmFilter := &listener.Filter{
		Name: "envoy.filters.network.http_connection_manager",
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: hcmConfig,
		},
	}
	lis.FilterChains[0].Filters = []*listener.Filter{hcmFilter}

	if lis.Name == InternalListener && config.InternalCert != nil {
		tlsLis, err := copyTLSListener(lis, config.InternalCert, InternalTLSPort)
		if err != nil {
			return err
		}
		listeners[tlsLis.Name] = types.ResourceWithTTL{Resource: tlsLis}
	}

	if err = resetSnapshot(types.Listener, items); err != nil {
		return err
	}
	return nil
}

func AddDefaultTimeout(action *route.RouteAction) *route.RouteAction {
	action.Timeout = &durationpb.Duration{}
	action.IdleTimeout = &durationpb.Duration{Seconds: int64(IdleTimeout)}
	action.MaxStreamDuration = &route.RouteAction_MaxStreamDuration{
		MaxStreamDuration:    &durationpb.Duration{},
		GrpcTimeoutHeaderMax: &durationpb.Duration{},
	}
	return action
}

func resetSnapshot(ty types.ResponseType, items map[string]types.ResourceWithTTL) error {
	oldVersion, _ := strconv.Atoi(snapshot.Resources[ty].Version)
	newVersion := fmt.Sprintf("%d", oldVersion+1)

	var clusterResources, routeResources, listenerResources []types.Resource
	if ty == types.Cluster {
		clusterResources = buildResourceFromResourcesItems(items)
	} else {
		clusterResources = buildResourcesFromSnapshot(types.Cluster)
	}

	if ty == types.Route {
		routeResources = buildResourceFromResourcesItems(items)
	} else {
		routeResources = buildResourcesFromSnapshot(types.Route)
	}

	if ty == types.Listener {
		listenerResources = buildResourceFromResourcesItems(items)
	} else {
		listenerResources = buildResourcesFromSnapshot(types.Listener)
	}

	newSnapshot, err := cache.NewSnapshot(newVersion, map[resource.Type][]types.Resource{
		resource.ClusterType:  clusterResources,
		resource.RouteType:    routeResources,
		resource.ListenerType: listenerResources,
	})
	if err != nil {
		return err
	}

	err = snapshotCache.SetSnapshot(ctx, nodeID, newSnapshot)
	if err != nil {
		return err
	}
	snapshot = newSnapshot
	return nil
}

func buildResourceFromResourcesItems(items map[string]types.ResourceWithTTL) []types.Resource {
	ret := make([]types.Resource, 0, len(items))
	for _, resourceWithTTL := range items {
		ret = append(ret, resourceWithTTL.Resource)
	}
	return ret
}

func buildResourcesFromSnapshot(ty types.ResponseType) []types.Resource {
	items := snapshot.Resources[ty].Items
	ret := make([]types.Resource, 0, len(items))
	for _, resourceWithTTL := range items {
		ret = append(ret, resourceWithTTL.Resource)
	}
	return ret
}
