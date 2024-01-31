// Package metric the function to export metrics to Prometheus
package promexporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"strings"
)

func ProduceCounter(namespace string, name string, help string, labels map[string]string) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}

func ProduceGauge(namespace string, name string, help string, labels map[string]string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}

func ProduceHistogram(namespace string, name string, help string, labels map[string]string) prometheus.Histogram {
	return prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   namespace,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
}
func ProduceSummary(namespace string, name string, help string, labels map[string]string) prometheus.Summary {
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

func ProduceMetric(reg *prometheus.Registry,
	metricId string, metricType string) *prometheus.Registry {
	splitedMetric := strings.Split(metricId, "__")
	labels := make(map[string]string)
	labels["type"] = "ss"
	labels["remote_domain"] = splitedMetric[len(splitedMetric)-3]
	name := splitedMetric[len(splitedMetric)-2]
	labels["aggregation_function"] = splitedMetric[len(splitedMetric)-1]
	help := name + " aggregated by " + labels["aggregation_function"] + " from ss"
	nameSpace := "ss"
	if metricType == "Counter" {
		counters[metricId] = ProduceCounter(nameSpace, name, help, labels)
		reg.MustRegister(counters[metricId])
	} else if metricType == "Gauge" {
		gauges[metricId] = ProduceGauge(nameSpace, name, help, labels)
		reg.MustRegister(gauges[metricId])
	} else if metricType == "Histogram" {
		histograms[metricId] = ProduceHistogram(nameSpace, name, help, labels)
		reg.MustRegister(histograms[metricId])
	} else if metricType == "Summary" {
		summaries[metricId] = ProduceSummary(nameSpace, name, help, labels)
		reg.MustRegister(summaries[metricId])
	}
	return reg
}
func Formalize(metric string) string {
	metric = strings.Replace(metric, "-", "_", -1)
	metric = strings.Replace(metric, ".", "__", -1)
	metric = strings.ToLower(metric)
	return metric
}
func ProduceMetrics(
	localDomainName string,
	clusterAddresses map[string][]string,
	netMetrics []string,
	MetricTypes map[string]string,
	aggregationMetrics map[string]string) *prometheus.Registry {
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
		metricId := Formalize(metric)
		splitedMetric := strings.Split(metric, ".")
		var metricTypeId string
		metricTypeId = splitedMetric[len(splitedMetric)-2]
		metricType, ok := MetricTypes[metricTypeId]
		if !ok {
			nlog.Error("Fail to get metric types", ok)
		}
		switch metricType {
		case "Counter":
			if _, ok := counters[metricId]; ok {
				counters[metricId].Add(val)
			} else {
				ProduceMetric(reg, metricId, metricType)
				counters[metricId].Add(val)
			}
		case "Gauge":
			if _, ok := gauges[metricId]; ok {
				gauges[metricId].Set(val)
			} else {
				ProduceMetric(reg, metricId, metricType)
				gauges[metricId].Set(val)
			}
		case "Histogram":
			if _, ok := histograms[metricId]; ok {
				histograms[metricId].Observe(val)
			} else {
				ProduceMetric(reg, metricId, metricType)
				histograms[metricId].Observe(val)
			}
		case "Summary":
			if _, ok := summaries[metricId]; ok {
				summaries[metricId].Observe(val)
			} else {
				ProduceMetric(reg, metricId, metricType)
				summaries[metricId].Observe(val)
			}
		}
	}
}
