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
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	gatewayLiveTimeout = 3 * time.Minute
)

func (c *controller) Monitorcdrstatus(ctx context.Context) {
	c.UpdateStatus(ctx)
}

func (c *controller) UpdateStatus(ctx context.Context) error {
	cdrs, err := c.clusterDomainRouteLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, cdr1 := range cdrs {
		update := false
		cdr := cdr1.DeepCopy()
		gws, err := c.gatewayLister.Gateways(cdr.Spec.Source).List(labels.Everything())
		if err != nil {
			nlog.Errorf("List %s's gateways failed with %v", cdr.Spec.Source, err)
			continue
		}
		healthyCount := 0
		if cdr.Status.EndpointStatuses == nil {
			cdr.Status.EndpointStatuses = map[string]kusciaapisv1alpha1.ClusterDomainRouteEndpointStatus{}
		}
		for _, gw := range gws {
			if time.Since(gw.Status.HeartbeatTime.Time) > gatewayLiveTimeout {
				continue
			}
			for _, metric := range gw.Status.NetworkStatus {
				if metric.Type != "DomainRoute" {
					continue
				}
				for _, port := range cdr.Spec.Endpoint.Ports {
					if metric.Name == fmt.Sprintf("%s-to-%s-%s", cdr.Spec.Source, cdr.Spec.Destination, port.Name) && metric.HealthyEndpointsCount > 0 {
						healthyCount++
						if v, ok := cdr.Status.EndpointStatuses[gw.Name+"-"+port.Name]; !ok || !v.EndpointHealthy {
							cdr.Status.EndpointStatuses[gw.Name+"-"+port.Name] = kusciaapisv1alpha1.ClusterDomainRouteEndpointStatus{
								EndpointHealthy: true,
							}
							update = true
						}
					}
				}
			}
		}

		if update {
			if _, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(ctx, cdr, metav1.UpdateOptions{}); err != nil {
				nlog.Warnf("Update cdr %s status failed with %v", cdr.Name, err)
			}
		}
	}

	return nil
}
