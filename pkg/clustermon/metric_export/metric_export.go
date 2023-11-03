package metric_export

import (
	"strings"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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
	metricId := nameSpace + "_" + name
	if metricType == "Counter" {
		counters[metricId] = ProduceCounter(nameSpace, name, help)
		reg.MustRegister(counters[metricId])
	} else if metricType == "Gauge" {
		gauges[metricId] = ProduceGauge(nameSpace, name, help)
		reg.MustRegister(gauges[metricId])
	} else if metricType == "Histogram" {
		histograms[metricId] = ProduceHistogram(nameSpace, name, help)
		reg.MustRegister(histograms[metricId])
	} else if metricType == "Summary" {
		summarys[metricId] = ProduceSummary(nameSpace, name, help)
		reg.MustRegister(summarys[metricId])
	}
	return reg
}
func Formalize(metric string) string{
	metric = strings.Replace(metric, "-", "_", -1)
	metric = strings.Replace(metric, ".", "__", -1)
	metric = strings.ToLower(metric)
	return metric
}
func ProduceMetrics(localDomainName string,
	clusterName string,
	destinationAddress []string,
	netMetrics []string,
	cluMetrics []string,
	MetricTypes map[string]string,
	aggregationMetrics map[string]string) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	clusterName = Formalize(clusterName)
	for _, dstAddr := range destinationAddress {
                dstDomain := Formalize(strings.Split(dstAddr, ":")[0])
                for _, metric := range netMetrics {
			metric = Formalize(metric)
			reg = ProduceMetric(reg, MetricTypes[metric], localDomainName + "__" + dstDomain+"_", metric + "__" + aggregationMetrics[metric], metric + "with aggregation function " + aggregationMetrics[metric] )
		}
        }

	for _, metric := range cluMetrics {
		metric = Formalize(metric)
		reg = ProduceMetric(reg, MetricTypes[metric], "cluster__" + clusterName+"_", metric, metric)
	}
	return reg
}

func UpdateMetrics(clusterResults map[string]map[string]float64, MetricTypes map[string]string) {
	for _, clusterResult := range clusterResults {
		for metric, val := range clusterResult {
			metricId := Formalize(metric)
			metricTypeId := strings.Join(strings.Split(metric, ".")[2: 3], "__")
			/*var metricTypeId string
			if strings.Split(metric, ".")[0] == "cluster"{
				metricTypeId = strings.Join(strings.Split(metric, ".")[2:], "__")
			}else{
				metricTypeId = strings.Join(strings.Split(metric, ".")[2: 3], "__")
			}*/
			switch MetricTypes[metricTypeId] {
			case "Counter":
				counters[metricId].Add(val)
			case "Gauge":
				gauges[metricId].Set(val)
			case "Histogram":
				histograms[metricId].Observe(val)
			case "Summary":
				summarys[metricId].Observe(val)
			}
		}
	}
}
