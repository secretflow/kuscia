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

package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	corelister "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/controller"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type metrics struct {
	Stats []metric `json:"stats"`
}

type metric struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type ClusterMetricsCollector struct {
	serviceLister     corelister.ServiceLister
	endpointsLister   corelister.EndpointsLister
	domainRouteLister kuscialistersv1alpha1.DomainRouteLister
	gc                *controller.GatewayController
	metricsEndpoint   string
}

func NewClusterMetricsCollector(sl corelister.ServiceLister, el corelister.EndpointsLister,
	drl kuscialistersv1alpha1.DomainRouteLister, gc *controller.GatewayController,
	envoyStatsEndpoint string) *ClusterMetricsCollector {
	return &ClusterMetricsCollector{
		serviceLister:     sl,
		endpointsLister:   el,
		domainRouteLister: drl,
		gc:                gc,
		metricsEndpoint:   fmt.Sprintf("%s/stats", envoyStatsEndpoint),
	}
}

func (c *ClusterMetricsCollector) MonitorClusterMetrics(stopCh <-chan struct{}) {
	go wait.Until(c.collect, 15*time.Second, stopCh)
}

func (c *ClusterMetricsCollector) collect() {
	metrics, err := c.getMetrics()
	if err != nil || metrics == nil {
		nlog.Warnf("get metrics from envoy failed with %v", err)
		return
	}

	var networkStatus []*kusciaapisv1alpha1.GatewayEndpointStatus
	if services, err := c.serviceLister.List(labels.Everything()); err == nil {
		for _, s := range services {
			if s.Spec.Type != v1.ServiceTypeExternalName {
				continue
			}
			total, ok := metrics[fmt.Sprintf("cluster.service-%s.membership_total", s.Name)]
			if !ok {
				nlog.Warnf("metric membership_total not found for %s", s.Name)
				continue
			}
			healthy, ok := metrics[fmt.Sprintf("cluster.service-%s.membership_healthy", s.Name)]
			if !ok {
				nlog.Warnf("metric membership_healthy not found for %s", s.Name)
				continue
			}
			networkStatus = append(networkStatus, &kusciaapisv1alpha1.GatewayEndpointStatus{
				Name:                  s.Name,
				Type:                  "Service",
				TotalEndpointsCount:   total,
				HealthyEndpointsCount: healthy,
			})
		}
	}

	if endpoints, err := c.endpointsLister.List(labels.Everything()); err == nil {
		for _, e := range endpoints {
			total, ok := metrics[fmt.Sprintf("cluster.service-%s.membership_total", e.Name)]
			if !ok {
				continue
			}
			healthy, ok := metrics[fmt.Sprintf("cluster.service-%s.membership_healthy", e.Name)]
			if !ok {
				continue
			}
			networkStatus = append(networkStatus, &kusciaapisv1alpha1.GatewayEndpointStatus{
				Name:                  e.Name,
				Type:                  "Endpoints",
				TotalEndpointsCount:   total,
				HealthyEndpointsCount: healthy,
			})
		}
	}

	if drs, err := c.domainRouteLister.List(labels.Everything()); err == nil {
		for _, dr := range drs {
			if dr.Spec.Source != dr.Namespace || utils.IsThirdPartyTransit(dr.Spec.Transit) {
				continue
			}

			for _, port := range dr.Spec.Endpoint.Ports {
				clusterName := common.GenerateClusterName(dr.Spec.Source, dr.Spec.Destination, port.Name)
				total, ok := metrics[fmt.Sprintf("cluster.%s.membership_total", clusterName)]
				if !ok {
					continue
				}
				healthy, ok := metrics[fmt.Sprintf("cluster.%s.membership_healthy", clusterName)]
				if !ok {
					continue
				}
				networkStatus = append(networkStatus, &kusciaapisv1alpha1.GatewayEndpointStatus{
					Name:                  clusterName,
					Type:                  "DomainRoute",
					TotalEndpointsCount:   total,
					HealthyEndpointsCount: healthy,
				})
			}
		}
	}

	if len(networkStatus) > 0 {
		c.gc.UpdateStatus(networkStatus)
	}
}

func (c *ClusterMetricsCollector) getMetrics() (map[string]int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?filter=membership_total|membership_healthy&format=json",
		c.metricsEndpoint),
		nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var metrics metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, metric := range metrics.Stats {
		result[metric.Name] = metric.Value
	}
	return result, nil
}
