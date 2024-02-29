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

// Package metric the function to export metrics to Prometheus
package promexporter

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func produceCounter(namespace string, name string, help string, labels map[string]string) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}

func produceGauge(namespace string, name string, help string, labels map[string]string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}

func produceHistogram(namespace string, name string, help string, labels map[string]string) prometheus.Histogram {
	return prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}
func produceSummary(namespace string, name string, help string, labels map[string]string) prometheus.Summary {
	return prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}

var counters = make(map[string]prometheus.Counter)
var gauges = make(map[string]prometheus.Gauge)
var histograms = make(map[string]prometheus.Histogram)
var summaries = make(map[string]prometheus.Summary)

func produceMetric(reg *prometheus.Registry,
	metricID string, metricType string) *prometheus.Registry {
	splitedMetric := strings.Split(metricID, ";")
	labels := make(map[string]string)
	labels["type"] = "ss"
	labels["remote_domain"] = splitedMetric[len(splitedMetric)-3]
	name := splitedMetric[len(splitedMetric)-2]
	labels["aggregation_function"] = splitedMetric[len(splitedMetric)-1]
	help := name + " aggregated by " + labels["aggregation_function"] + " from ss"
	nameSpace := "ss"
	if metricType == "Counter" {
		counters[metricID] = produceCounter(nameSpace, name, help, labels)
		reg.MustRegister(counters[metricID])
	} else if metricType == "Gauge" {
		gauges[metricID] = produceGauge(nameSpace, name, help, labels)
		reg.MustRegister(gauges[metricID])
	} else if metricType == "Histogram" {
		histograms[metricID] = produceHistogram(nameSpace, name, help, labels)
		reg.MustRegister(histograms[metricID])
	} else if metricType == "Summary" {
		summaries[metricID] = produceSummary(nameSpace, name, help, labels)
		reg.MustRegister(summaries[metricID])
	}
	return reg
}
func formalize(metric string) string {
	metric = strings.Replace(metric, "-", "_", -1)
	metric = strings.Replace(metric, ".", "__", -1)
	metric = strings.ToLower(metric)
	return metric
}
func ProduceRegister() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

func UpdateMetrics(reg *prometheus.Registry,
	clusterResults map[string]float64, MetricTypes map[string]string) {
	for metric, val := range clusterResults {
		metricID := formalize(metric)
		splitedMetric := strings.Split(metric, ";")
		var metricTypeID string
		metricTypeID = splitedMetric[len(splitedMetric)-2]
		metricType, ok := MetricTypes[metricTypeID]
		if !ok {
			nlog.Error("Fail to get metric types", ok)
		}
		switch metricType {
		case "Counter":
			if _, ok := counters[metricID]; !ok {
				produceMetric(reg, metricID, metricType)
			}
			counters[metricID].Add(val)
		case "Gauge":
			if _, ok := gauges[metricID]; !ok {
				produceMetric(reg, metricID, metricType)
			}
			gauges[metricID].Set(val)
		case "Histogram":
			if _, ok := histograms[metricID]; !ok {
				produceMetric(reg, metricID, metricType)
			}
			histograms[metricID].Observe(val)
		case "Summary":
			if _, ok := summaries[metricID]; !ok {
				produceMetric(reg, metricID, metricType)
			}
			summaries[metricID].Observe(val)
		}
	}
}
