package prome_regist

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
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

func Formalize(metric string) string {
	metric = strings.Replace(metric, "-", "_", -1)
	metric = strings.Replace(metric, ".", "__", -1)
	metric = strings.ToLower(metric)
	return metric
}
