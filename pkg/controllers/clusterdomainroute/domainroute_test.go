// Copyright 2024 Ant Group Co., Ltd.
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

package clusterdomainroute

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestTokenHeartBeatTimeout(t *testing.T) {
	testCases := []struct {
		name    string
		timeout bool
		tokens  []kusciaapisv1alpha1.DomainRouteToken
	}{
		{
			name:    "empty tokens",
			timeout: false,
			tokens:  []kusciaapisv1alpha1.DomainRouteToken{},
		},
		{
			name:    "no ready token exists",
			timeout: false,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: false,
				},
			},
		},
		{
			name:    "no heartbeat time exists",
			timeout: false,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: true,
				},
			},
		},
		{
			name:    "heartbeat time exists but no timeout",
			timeout: false,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: false,
				},
				{
					IsReady: true,
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.Now(),
				},
			},
		},
		{
			name:    "heartbeat time exists and no timeout exists",
			timeout: false,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: false,
				},
				{
					IsReady: true,
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.Now(),
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.NewTime(time.Now().Add(-3 * common.GatewayHealthCheckDuration)),
				},
			},
		},
		{
			name:    "heartbeat time exists and timeout exists",
			timeout: true,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: false,
				},
				{
					IsReady: true,
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.NewTime(time.Now().Add(-3 * common.GatewayHealthCheckDuration)),
				},
			},
		},
		{
			name:    "heartbeat time exists and no timeout exists",
			timeout: false,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: true,
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.Now(),
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.NewTime(time.Now().Add(-3 * common.GatewayHealthCheckDuration)),
				},
			},
		},
		{
			name:    "heartbeat time exists but timeout",
			timeout: true,
			tokens: []kusciaapisv1alpha1.DomainRouteToken{
				{
					IsReady: true,
				},
				{
					IsReady:       false,
					HeartBeatTime: metav1.Now(),
				},
				{
					IsReady:       true,
					HeartBeatTime: metav1.NewTime(time.Now().Add(-3 * common.GatewayHealthCheckDuration)),
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, IsTokenHeartBeatTimeout(testCase.tokens), testCase.timeout)
		})
	}
}
