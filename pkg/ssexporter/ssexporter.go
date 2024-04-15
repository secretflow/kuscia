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

package ssexporter

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/ssexporter/parse"
	"github.com/secretflow/kuscia/pkg/ssexporter/promexporter"
	"github.com/secretflow/kuscia/pkg/ssexporter/ssmetrics"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func SsExporter(ctx context.Context, runMode pkgcom.RunModeType, domainID string, exportPeriod uint, port string) error {
	// read the config
	_, AggregationMetrics := parse.LoadMetricConfig()
	clusterAddresses, _ := parse.GetClusterAddress(domainID)
	localDomainName := domainID
	var MetricTypes = promexporter.NewMetricTypes()

	reg := promexporter.ProduceRegister()
	lastClusterMetricValues, err := ssmetrics.GetSsMetricResults(runMode, localDomainName, clusterAddresses, AggregationMetrics, exportPeriod)
	if err != nil {
		nlog.Warnf("Fail to get ss metric results, err: %v", err)
		return err
	}
	// export the cluster metrics
	ticker := time.NewTicker(time.Duration(exportPeriod) * time.Second)
	defer ticker.Stop()
	go func(runMode pkgcom.RunModeType, reg *prometheus.Registry, MetricTypes map[string]string, exportPeriods uint, lastClusterMetricValues map[string]float64) {
		for range ticker.C {
			// get clusterName and clusterAddress
			clusterAddresses, _ = parse.GetClusterAddress(domainID)
			// get cluster metrics
			currentClusterMetricValues, err := ssmetrics.GetSsMetricResults(runMode, localDomainName, clusterAddresses, AggregationMetrics, exportPeriods)
			if err != nil {
				nlog.Warnf("Fail to get ss metric results, err: %v", err)
			}
			// calculate the change values of cluster metrics
			lastClusterMetricValues, currentClusterMetricValues = ssmetrics.GetMetricChange(lastClusterMetricValues, currentClusterMetricValues)
			// update cluster metrics in prometheus
			promexporter.UpdateMetrics(reg, currentClusterMetricValues, MetricTypes)
		}
	}(runMode, reg, MetricTypes, exportPeriod, lastClusterMetricValues)
	// export to the prometheus
	ssServer := http.NewServeMux()
	ssServer.Handle("/ssmetrics", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}))
	go func() {
		if err := http.ListenAndServe("0.0.0.0:"+port, ssServer); err != nil {
			nlog.Error("Fail to start the metric exporterserver", err)
		}
	}()
	defer func() {
		close(ReadyChan)
		nlog.Info("Start to export metrics...")
	}()

	<-ctx.Done()
	nlog.Info("Stopping the metric exporter...")
	return nil
}
