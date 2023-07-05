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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	grpc_http1_reverse_bridge "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_reverse_bridge/v3"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/gateway/controller/interconn"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	processPeriod      = time.Second
	defaultSyncPeriod  = 10 * time.Minute
	endpointsQueueName = "endpoints-queue"
)

const (
	portScopeDomain  = "Domain"
	portScopeCluster = "Cluster"
	portScopeLocal   = "Local"
)

const (
	protocolHTTP  = "HTTP"
	protocolHTTPS = "HTTPS"
	protocolGRPC  = "GRPC"
	protocolGRPCS = "GRPCS"
)

var (
	keyFunc         = cache.DeletionHandlingMetaNamespaceKeyFunc
	serviceProtocol = map[string]bool{
		protocolHTTP:  true,
		protocolHTTPS: true,
		protocolGRPC:  true,
		protocolGRPCS: true,
	}
)

type EndpointsController struct {
	serviceLister       corelisters.ServiceLister
	serviceListerSynced cache.InformerSynced

	endpointsLister       corelisters.EndpointsLister
	endpointsListerSynced cache.InformerSynced
	queue                 workqueue.RateLimitingInterface
	whitelistChecker      utils.WhitelistChecker
	clientCert            *xds.TLSCert
}

func NewEndpointsController(serviceInformer corev1informers.ServiceInformer,
	endpointsInformer corev1informers.EndpointsInformer, whitelistFile string,
	clientCert *xds.TLSCert) (*EndpointsController, error) {
	whitelistChecker, err := utils.NewWhitelistChecker(whitelistFile)
	if err != nil {
		nlog.Warnf("New whitelist failed with %v", err)
		return nil, err
	}

	ec := &EndpointsController{
		serviceLister:         serviceInformer.Lister(),
		serviceListerSynced:   serviceInformer.Informer().HasSynced,
		endpointsLister:       endpointsInformer.Lister(),
		endpointsListerSynced: endpointsInformer.Informer().HasSynced,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "endpoints"),
		whitelistChecker:      whitelistChecker,
		clientCert:            clientCert,
	}

	ec.addServiceEventHandler(serviceInformer)
	ec.addEndpointsEventHandler(endpointsInformer)

	return ec, nil
}

func (ec *EndpointsController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ec.queue.ShutDown()

	nlog.Info("Waiting for informer caches to sync")
	if !cache.WaitForNamedCacheSync("endpoints", stopCh, ec.serviceListerSynced, ec.endpointsListerSynced) {
		nlog.Fatal("failed to wait for caches to sync")
	}

	nlog.Info("Starting endpoints Controller ")
	for i := 0; i < workers; i++ {
		go wait.Until(ec.worker, processPeriod, stopCh)
	}

	<-stopCh
	nlog.Info("Shutting down endpoints Controller")
}

func (ec *EndpointsController) addServiceEventHandler(serviceInformer corev1informers.ServiceInformer) {
	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				svc, ok := obj.(*v1.Service)
				if ok && svc.Spec.Type == v1.ServiceTypeExternalName {
					return true
				}
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: ec.updateService,
				UpdateFunc: func(oldObj, newObj interface{}) {
					ec.updateService(newObj)
				},
				DeleteFunc: ec.updateService,
			},
		},
		defaultSyncPeriod,
	)
}

func (ec *EndpointsController) addEndpointsEventHandler(endpointInformer corev1informers.EndpointsInformer) {
	endpointInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: ec.updateEndpoints,
			UpdateFunc: func(oldObj, newObj interface{}) {
				ec.updateEndpoints(newObj)
			},
			DeleteFunc: ec.updateEndpoints,
		},
		defaultSyncPeriod,
	)
}

func (ec *EndpointsController) updateService(obj interface{}) {
	svc, ok := obj.(*v1.Service)
	if !ok {
		nlog.Warnf("interface{} is %T, not *v1.Service", obj)
		return
	}

	nlog.Infof("Updating service %s/%s/%s", svc.Namespace, svc.Name, svc.ResourceVersion)
	ec.enqueue(svc)
}

func (ec *EndpointsController) updateEndpoints(obj interface{}) {
	ep, ok := obj.(*v1.Endpoints)
	if !ok {
		nlog.Warnf("interface{} is %T, not *v1.Endpoints", obj)
		return
	}

	nlog.Infof("Updating endpoint %s/%s/%s", ep.Namespace, ep.Name, ep.ResourceVersion)
	ec.enqueue(ep)
}

func (ec *EndpointsController) enqueue(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, ec.queue)
}

func (ec *EndpointsController) worker() {
	for queue.HandleQueueItem(context.Background(), endpointsQueueName, ec.queue, ec.syncHandler, maxRetries) {
	}
}

func (ec *EndpointsController) syncHandler(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		nlog.Debugf("Finished syncing endpoints %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	service, err := ec.serviceLister.Services(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("unable to retrieve service %v from store %v", key, err))
		return err
	}
	if service == nil {
		return deleteService(name)
	}

	portScope := service.Labels[common.LabelPortScope]
	var accessDomains string
	if portScope == portScopeDomain {
		accessDomains = namespace
	} else {
		accessDomains = service.Annotations[common.AccessDomainAnnotationKey]
	}
	protocol, err := parseAndValidateProtocol(service.Annotations[common.ProtocolAnnotationKey], key)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	if service.Spec.Type == v1.ServiceTypeExternalName {
		return ec.AddEnvoyClusterByExternalName(service, protocol, namespace, name, accessDomains)
	}

	endpoints, err := ec.endpointsLister.Endpoints(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("unable to retrieve endpoints %v from store: %v", key, err))
		return err
	}
	if endpoints == nil {
		return deleteService(name)
	}

	return ec.AddEnvoyClusterByEndpoints(endpoints, protocol, namespace, name, accessDomains)
}

func (ec *EndpointsController) AddEnvoyClusterByExternalName(service *v1.Service, protocol string, namespace string,
	name string, accessDomains string) error {
	var ports []uint32
	for _, port := range service.Spec.Ports {
		ports = append(ports, uint32(port.Port))
	}
	err := ec.validateAddress(service.Spec.ExternalName, ports)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	hosts := make(map[string][]uint32)
	hosts[service.Spec.ExternalName] = ports
	return AddEnvoyCluster(namespace, name, protocol, hosts, accessDomains, ec.clientCert)
}

func (ec *EndpointsController) AddEnvoyClusterByEndpoints(endpoints *v1.Endpoints, protocol string, namespace string,
	name string, accessDomains string) error {
	hosts := make(map[string][]uint32)
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			var ports []uint32
			for _, port := range subset.Ports {
				ports = append(ports, uint32(port.Port))
			}

			err := ec.validateAddress(address.IP, ports)
			if err != nil {
				utilruntime.HandleError(err)
				continue
			}

			hosts[address.IP] = ports
		}
	}
	if len(hosts) == 0 {
		return nil
	}

	return AddEnvoyCluster(namespace, name, protocol, hosts, accessDomains, ec.clientCert)
}

func deleteService(name string) error {
	if err := xds.DeleteVirtualHost(fmt.Sprintf("service-%s-internal", name), xds.InternalRoute); err != nil {
		return fmt.Errorf("delete virtual host %s failed with %v", name, err)
	}
	if err := xds.DeleteVirtualHost(fmt.Sprintf("service-%s-external", name), xds.ExternalRoute); err != nil {
		return fmt.Errorf("delete virtual host %s failed with %v", name, err)
	}
	if err := xds.DeleteCluster(fmt.Sprintf("service-%s", name)); err != nil {
		return fmt.Errorf("delete cluster %s failed with %v", name, err)
	}
	return nil
}

func (ec *EndpointsController) validateAddress(address string, ports []uint32) error {
	if ec.whitelistChecker != nil && !reflect.ValueOf(ec.whitelistChecker).IsNil() && !ec.whitelistChecker.Check(address, ports) {
		err := fmt.Errorf("%s is not in whitelist, please check it", address)
		nlog.Error(err)
		return err
	}
	return nil
}

func parseAndValidateProtocol(protocol string, service string) (string, error) {
	if protocol == "" {
		protocol = protocolHTTP
	}
	if !serviceProtocol[protocol] {
		err := fmt.Errorf("unsupported service protocol: %s, service: %s", protocol, service)
		return protocol, err
	}
	return protocol, nil
}

func AddEnvoyCluster(namespace string, name string, protocol string, hosts map[string][]uint32,
	accessDomains string, clientCert *xds.TLSCert) error {
	internalVh, err := generateVirtualHost(namespace, name, accessDomains)
	if err != nil {
		return err
	}

	externalVh, ok := proto.Clone(internalVh).(*route.VirtualHost)
	if !ok {
		return fmt.Errorf("internalVh proto cannot cast to VirtualHost")
	}
	decorateInternalVirtualHost(internalVh, name)
	decorateExternalVirtualHost(externalVh, name)

	cluster, err := generateCluster(name, protocol, hosts, clientCert)
	if err != nil {
		return err
	}

	if err := xds.AddOrUpdateCluster(cluster); err != nil {
		return err
	}

	if err := xds.AddOrUpdateVirtualHost(internalVh, xds.InternalRoute); err != nil {
		return err
	}

	if err := xds.AddOrUpdateVirtualHost(externalVh, xds.ExternalRoute); err != nil {
		return err
	}

	return nil
}

func generateVirtualHost(namespace string, name string, accessDomains string) (*route.VirtualHost, error) {
	virtualHost := &route.VirtualHost{
		Domains: []string{fmt.Sprintf("%s.%s.svc", name, namespace), name},
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
								Cluster: fmt.Sprintf("service-%s", name),
							},
							HostRewriteSpecifier: &route.RouteAction_AutoHostRewrite{
								AutoHostRewrite: wrapperspb.Bool(true),
							},
							HashPolicy: []*route.RouteAction_HashPolicy{
								{
									PolicySpecifier: &route.RouteAction_HashPolicy_Header_{
										Header: &route.RouteAction_HashPolicy_Header{
											HeaderName: interconn.GetHashHeaderOfService(name),
										},
									},
								},
							},
						},
					),
				},
			},
		},
	}

	// accessDomainRegex for example "(node-alice|node-bob|node-joke)"
	if len(accessDomains) > 0 {
		domains := strings.Replace(accessDomains, ",", "|", -1)
		accessDomainRegex := fmt.Sprintf("(%s)", domains)
		virtualHost.Routes[0].Match.Headers = []*route.HeaderMatcher{
			{
				Name: "Kuscia-Source",
				HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
					StringMatch: &matcher.StringMatcher{
						MatchPattern: &matcher.StringMatcher_SafeRegex{
							SafeRegex: &matcher.RegexMatcher{
								EngineType: &matcher.RegexMatcher_GoogleRe2{},
								Regex:      accessDomainRegex,
							},
						},
					},
				},
			},
		}
	}
	return virtualHost, nil
}

func decorateExternalVirtualHost(vh *route.VirtualHost, name string) {
	vh.Name = fmt.Sprintf("service-%s-external", name)
}

func decorateInternalVirtualHost(vh *route.VirtualHost, name string) {
	vh.Name = fmt.Sprintf("service-%s-internal", name)
	b, _ := proto.Marshal(&grpc_http1_reverse_bridge.FilterConfigPerRoute{
		Disabled: true,
	})
	vh.Routes[0].TypedPerFilterConfig = map[string]*anypb.Any{
		"envoy.filters.http.grpc_http1_reverse_bridge": {
			TypeUrl: "type.googleapis.com/envoy.extensions.filters.http.grpc_http1_reverse_bridge.v3.FilterConfigPerRoute",
			Value:   b,
		},
	}
}

func generateCluster(name string, protocol string, hosts map[string][]uint32,
	clientCert *xds.TLSCert) (*envoycluster.Cluster, error) {
	var endpoints []*endpoint.LbEndpoint
	for host, ports := range hosts {
		for _, port := range ports {
			endpoints = append(endpoints, &endpoint.LbEndpoint{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Address: host,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: port,
									},
								},
							},
						},
						Hostname: host,
					},
				},
			})
		}
	}

	var protocolOptions *envoyhttp.HttpProtocolOptions
	if protocol == protocolGRPC || protocol == protocolGRPCS {
		protocolOptions = &envoyhttp.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
				},
			},
		}
	} else {
		protocolOptions = &envoyhttp.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoyhttp.HttpProtocolOptions_UseDownstreamProtocolConfig{
				UseDownstreamProtocolConfig: &envoyhttp.HttpProtocolOptions_UseDownstreamHttpConfig{},
			},
		}
	}

	b, err := proto.Marshal(protocolOptions)
	if err != nil {
		nlog.Errorf("Marshal protocolOptions failed with %s", err.Error())
		return nil, err
	}

	cluster := xds.AddTCPHealthCheck(&envoycluster.Cluster{
		Name:           fmt.Sprintf("service-%s", name),
		ConnectTimeout: durationpb.New(10 * time.Second),
		ClusterDiscoveryType: &envoycluster.Cluster_Type{
			Type: envoycluster.Cluster_STRICT_DNS,
		},
		LbPolicy: envoycluster.Cluster_RING_HASH,
		LbConfig: &envoycluster.Cluster_RingHashLbConfig_{
			RingHashLbConfig: &envoycluster.Cluster_RingHashLbConfig{
				HashFunction: envoycluster.Cluster_RingHashLbConfig_MURMUR_HASH_2,
			},
		},
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: fmt.Sprintf("service-%s", name),
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: endpoints,
				},
			},
		},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": {
				TypeUrl: "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
				Value:   b,
			},
		},
	})

	if clientCert != nil {
		cluster.TransportSocket, err = xds.GenerateUpstreamTLSConfigByCert(clientCert)
		if err != nil {
			return cluster, err
		}
	}

	if protocol == protocolHTTPS || protocol == protocolGRPCS {
		cluster.TransportSocket = &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
		}
	}
	return cluster, nil
}
