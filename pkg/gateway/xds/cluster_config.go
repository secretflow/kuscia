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
	"time"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func AddTCPHealthCheck(c *envoycluster.Cluster) *envoycluster.Cluster {
	c.CommonLbConfig = &envoycluster.Cluster_CommonLbConfig{
		HealthyPanicThreshold: &v3.Percent{
			Value: 5,
		},
	}
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
	c.OutlierDetection = &envoycluster.OutlierDetection{
		EnforcingConsecutive_5Xx:       wrapperspb.UInt32(0), // disable this, we only count local origin failures
		SplitExternalLocalOriginErrors: true,
		ConsecutiveLocalOriginFailure:  wrapperspb.UInt32(1),
		BaseEjectionTime:               durationpb.New(30 * time.Second),
		MaxEjectionTime:                durationpb.New(30 * time.Second),
	}
	return c
}
