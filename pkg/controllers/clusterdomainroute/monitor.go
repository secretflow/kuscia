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
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
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
		if cdr.Status.EndpointStatuses == nil {
			cdr.Status.EndpointStatuses = map[string]kusciaapisv1alpha1.ClusterDomainRouteEndpointStatus{}
		}
		endpointsHealthy := map[string]bool{}
		for _, gw := range gws {
			if time.Since(gw.Status.HeartbeatTime.Time) > common.GatewayLiveTimeout {
				continue
			}
			for _, port := range cdr.Spec.Endpoint.Ports {
				expectMetricsName := common.GenerateClusterName(cdr.Spec.Source, cdr.Spec.Destination, port.Name)
				for _, metric := range gw.Status.NetworkStatus {
					if metric.Type != "DomainRoute" {
						continue
					}
					if metric.Name == expectMetricsName && metric.HealthyEndpointsCount > 0 {
						endpointsHealthy[gw.Name+"-"+port.Name] = true
					}
				}
			}
		}
		for k, eh := range endpointsHealthy {
			if v, ok := cdr.Status.EndpointStatuses[k]; !ok || !v.EndpointHealthy {
				cdr.Status.EndpointStatuses[k] = kusciaapisv1alpha1.ClusterDomainRouteEndpointStatus{
					EndpointHealthy: eh,
				}
				update = true
			}
		}
		for k, es := range cdr.Status.EndpointStatuses {
			if _, ok := endpointsHealthy[k]; !ok && es.EndpointHealthy {
				cdr.Status.EndpointStatuses[k] = kusciaapisv1alpha1.ClusterDomainRouteEndpointStatus{
					EndpointHealthy: false,
				}
				update = true
			}
		}

		if update {
			_, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(ctx, cdr, metav1.UpdateOptions{})
			if err != nil && !k8serrors.IsConflict(err) {
				nlog.Warnf("Update cdr %s status failed with %v", cdr.Name, err)
			}
			if err == nil {
				nlog.Infof("ClusterDomainRoute %s update monitor status", cdr.Name)
			}
		}
	}

	return nil
}
