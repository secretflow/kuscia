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

package clusterdomainroute

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func compareSpec(cdr *kusciaapisv1alpha1.ClusterDomainRoute, dr *kusciaapisv1alpha1.DomainRoute) bool {
	if !reflect.DeepEqual(cdr.Labels, dr.Labels) {
		return false
	}

	if !reflect.DeepEqual(cdr.Spec.DomainRouteSpec, dr.Spec) {
		return false
	}

	return true
}

func compareTokens(tokens1, tokens2 []kusciaapisv1alpha1.DomainRouteToken) bool {
	if len(tokens1) != len(tokens2) {
		return false
	}

	for i := 0; i < len(tokens1); i++ {
		if tokens1[i].Revision != tokens2[i].Revision {
			return false
		}
	}
	return true
}

func checkReadyCondition(cdr *kusciaapisv1alpha1.ClusterDomainRoute) bool {
	n := len(cdr.Status.Conditions)
	for i := 0; i < n; i++ {
		cond := &cdr.Status.Conditions[i]
		if cond.Type == kusciaapisv1alpha1.ClusterDomainRouteReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

func (c *Controller) checkEffectiveInstances(dr *kusciaapisv1alpha1.DomainRoute) bool {
	if len(dr.Status.TokenStatus.Tokens) == 0 {
		nlog.Error("dr tokens is nil")
		return false
	}

	gateways, err := c.gatewayLister.Gateways(dr.Namespace).List(labels.Everything())
	if err != nil {
		nlog.Errorf("List gateways failed with %v", err)
		return false
	}
	if gateways == nil {
		nlog.Error("gateways is null, please deploy first")
		return false
	}
	liveGateways := map[string]bool{}
	for i, gw := range gateways {
		if time.Since(gw.Status.HeartbeatTime.Time) < gatewayLiveTimeout {
			liveGateways[gateways[i].Name] = true
		}
	}

	found := 0
	n := len(dr.Status.TokenStatus.Tokens)
	for _, ins := range dr.Status.TokenStatus.Tokens[n-1].EffectiveInstances {
		if liveGateways[ins] {
			found++
		}
	}

	return found == len(liveGateways)
}
