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

package xds

import (
	"fmt"
	"strings"
	"time"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	ProtocolHTTP  = "HTTP"
	ProtocolHTTPS = "HTTPS"
	ProtocolGRPC  = "GRPC"
	ProtocolGRPCS = "GRPCS"
)

func GenerateProtocol(isTLS, isGRPC bool) string {
	if isTLS {
		if isGRPC {
			return ProtocolGRPCS
		}
		return ProtocolHTTPS
	}

	if isGRPC {
		return ProtocolGRPC
	}
	return ProtocolHTTP
}

func DecorateCluster(cluster *envoycluster.Cluster) {
	cluster.ConnectTimeout = durationpb.New(10 * time.Second)

	// dns lookup
	cluster.ClusterDiscoveryType = &envoycluster.Cluster_Type{
		Type: envoycluster.Cluster_STRICT_DNS,
	}
	cluster.DnsLookupFamily = envoycluster.Cluster_V4_ONLY

	// load balance policy
	cluster.LbPolicy = envoycluster.Cluster_RING_HASH
	cluster.LbConfig = &envoycluster.Cluster_RingHashLbConfig_{
		RingHashLbConfig: &envoycluster.Cluster_RingHashLbConfig{
			HashFunction: envoycluster.Cluster_RingHashLbConfig_MURMUR_HASH_2,
		},
	}

	cluster.CommonLbConfig = &envoycluster.Cluster_CommonLbConfig{
		HealthyPanicThreshold: &v3.Percent{
			Value: 5,
		},
	}

	cluster.OutlierDetection = &envoycluster.OutlierDetection{
		EnforcingConsecutive_5Xx:       wrapperspb.UInt32(0), // disable this, we only count local origin failures
		SplitExternalLocalOriginErrors: true,
		ConsecutiveLocalOriginFailure:  wrapperspb.UInt32(1),
		BaseEjectionTime:               durationpb.New(30 * time.Second),
		MaxEjectionTime:                durationpb.New(30 * time.Second),
	}
}

func DecorateRemoteUpstreamCluster(cluster *envoycluster.Cluster, protocol string) error {
	DecorateCluster(cluster)

	// enable tcp keep alive as a client
	cluster.UpstreamConnectionOptions = &envoycluster.UpstreamConnectionOptions{
		TcpKeepalive: &core.TcpKeepalive{},
	}

	// set HTTPOptions
	if err := GenerateUpstreamHTTPOptions(cluster, protocol); err != nil {
		return err
	}

	return DecorateClusterTransport(cluster, protocol)
}

func DecorateLocalUpstreamCluster(cluster *envoycluster.Cluster, protocol string) error {
	DecorateCluster(cluster)

	// set HTTPOptions
	if err := GenerateUpstreamHTTPOptions(cluster, protocol); err != nil {
		return err
	}

	// set transport
	return DecorateClusterTransport(cluster, protocol)
}

func DecorateClusterTransport(cluster *envoycluster.Cluster, protocol string) error {
	if cluster.TransportSocket != nil {
		return nil
	}

	protocol = strings.ToUpper(protocol)
	if protocol != ProtocolGRPCS && protocol != ProtocolHTTPS {
		return nil
	}

	tlsContext := &tls.UpstreamTlsContext{}
	conf, err := anypb.New(tlsContext)
	if err != nil {
		return fmt.Errorf("MarshalL UpstreamTlsContext failed with %s", err.Error())
	}
	nlog.Infof("Generate tls config for %s", cluster.Name)
	cluster.TransportSocket = &core.TransportSocket{
		Name: "envoy.transport_sockets.tls",
		ConfigType: &core.TransportSocket_TypedConfig{
			TypedConfig: conf,
		},
	}
	return nil
}

func GenerateUpstreamHTTPOptions(cluster *envoycluster.Cluster, protocol string) error {
	if cluster.TypedExtensionProtocolOptions == nil {
		cluster.TypedExtensionProtocolOptions = make(map[string]*anypb.Any)
	}
	HTTPOptions := "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
	if _, ok := cluster.TypedExtensionProtocolOptions[HTTPOptions]; ok {
		return nil
	}

	protocol = strings.ToUpper(protocol)

	var err error
	var opts *anypb.Any

	// set HTTPOptions
	if protocol == ProtocolGRPC || protocol == ProtocolGRPCS {
		opts, err = anypb.New(GenerateHTTP2UpstreamHTTPOptions(false))
	} else {
		opts, err = anypb.New(GenerateSimpleUpstreamHTTPOptions(false))
	}
	if err != nil {
		return err
	}

	cluster.TypedExtensionProtocolOptions[HTTPOptions] = opts
	return nil
}

func GenerateSimpleUpstreamHTTPOptions(isRemoteCluster bool) *envoyhttp.HttpProtocolOptions {
	protocolOptions := &envoyhttp.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoyhttp.HttpProtocolOptions_UseDownstreamProtocolConfig{
			UseDownstreamProtocolConfig: &envoyhttp.HttpProtocolOptions_UseDownstreamHttpConfig{},
		},
	}
	if isRemoteCluster {
		SetCommonHTTPProtocolOptions(protocolOptions)
	}
	return protocolOptions
}

func GenerateHTTP2UpstreamHTTPOptions(isRemoteCluster bool) *envoyhttp.HttpProtocolOptions {
	protocolOptions := &envoyhttp.HttpProtocolOptions{
		UpstreamProtocolOptions: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &envoyhttp.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
			},
		},
	}
	if isRemoteCluster {
		SetCommonHTTPProtocolOptions(protocolOptions)
	}
	return protocolOptions
}

func SetCommonHTTPProtocolOptions(options *envoyhttp.HttpProtocolOptions) {
	// set connections idle timeout
	options.CommonHttpProtocolOptions = &core.HttpProtocolOptions{
		IdleTimeout: &durationpb.Duration{Seconds: int64(900)},
	}
}

func AddTCPHealthCheck(c *envoycluster.Cluster) *envoycluster.Cluster {
	c.HealthChecks = []*core.HealthCheck{
		{
			Timeout:            durationpb.New(time.Second),
			Interval:           durationpb.New(15 * time.Second),
			UnhealthyInterval:  durationpb.New(3 * time.Second),
			UnhealthyThreshold: wrapperspb.UInt32(1),
			HealthyThreshold:   wrapperspb.UInt32(1),
			HealthChecker: &core.HealthCheck_TcpHealthCheck_{
				TcpHealthCheck: &core.HealthCheck_TcpHealthCheck{},
			},
		},
	}
	return c
}

func GetClusterHTTPProtocolOptions(clusterName string) (*envoyhttp.HttpProtocolOptions, *envoycluster.Cluster, error) {
	cluster, err := QueryCluster(clusterName)
	if err != nil {
		return nil, nil, err
	}

	option, ok := cluster.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]
	if !ok {
		return nil, cluster, err
	}

	var protocolOptions envoyhttp.HttpProtocolOptions
	if err := proto.Unmarshal(option.Value, &protocolOptions); err != nil {
		return nil, cluster, err
	}
	return &protocolOptions, cluster, nil
}
