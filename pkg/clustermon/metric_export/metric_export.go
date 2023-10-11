package metric_export

import (
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	//"fmt"
	"github.com/prometheus/client_golang/prometheus/collectors"
	//"E2EMon/metric_types"
	// "github.com/prometheus/client_golang/prometheus/promhttp"
	// "net/http"
)

func ProduceCounter(namespace string, name string, help string) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      name,
			Help:      help,
		})
}

func ProduceGauge(namespace string, name string, help string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      name,
			Help:      help,
			ConstLabels: map[string]string{
				"path": "/api/test",
			},
		})
}

func ProduceHistogram(namespace string, name string, help string) prometheus.Histogram {
	return prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      name,
			Help:      help,
			ConstLabels: map[string]string{
				"path": "/api/test",
			},
		})
}
func ProduceSummary(namespace string, name string, help string) prometheus.Summary {
	return prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      name,
			Help:      help,
		})
}

var counters = make(map[string]prometheus.Counter)
var gauges = make(map[string]prometheus.Gauge)
var histograms = make(map[string]prometheus.Histogram)
var summarys = make(map[string]prometheus.Summary)

func ProduceMetric(reg *prometheus.Registry,
	metricType string, nameSpace string, name string, help string) *prometheus.Registry {
	if metricType == "Counter" {
		counters[name] = ProduceCounter(nameSpace, name, help)
		reg.MustRegister(counters[name])
	} else if metricType == "Gauge" {
		gauges[name] = ProduceGauge(nameSpace, name, help)
		reg.MustRegister(gauges[name])
	} else if metricType == "Histogram" {
		histograms[name] = ProduceHistogram(nameSpace, name, help)
		reg.MustRegister(histograms[name])
	} else if metricType == "Summary" {
		summarys[name] = ProduceSummary(nameSpace, name, help)
		reg.MustRegister(summarys[name])
	}
	return reg
}
func ProduceMetrics(clusterName string,
	netMetrics []string,
	cluMetrics []string,
	MetricTypes map[string]string) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	clusterName = strings.Replace(clusterName, "-", "_", -1)
	for _, metric := range netMetrics {
		reg = ProduceMetric(reg, MetricTypes[metric], clusterName, metric, metric)
	}
	for _, metric := range cluMetrics {
		str := strings.Replace(metric, ".", "_", -1)
		str = strings.ToLower(str)
		reg = ProduceMetric(reg, MetricTypes[metric], clusterName, str, str)
	}

	return reg
}

func UpdateMetrics( /*counters map[string]prometheus.Counter,
	  gauges map[string]prometheus.Gauge,
	  histograms map[string]prometheus.Histogram,
	  summarys map[string]prometheus.Summary,*/
	clusterResults map[string]map[string]float64, MetricTypes map[string]string) {
	for _, clusterResult := range clusterResults {
		for metric, val := range clusterResult {
			switch MetricTypes[metric] {
			case "Counter":
				counters[metric].Add(val)
			case "Gauge":
				gauges[metric].Set(val)
			case "Histogram":
				histograms[metric].Observe(val)
			case "Summary":
				summarys[metric].Observe(val)
			}
		}
	}
}
