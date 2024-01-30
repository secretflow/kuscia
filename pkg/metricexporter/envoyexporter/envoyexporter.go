package envoyexporter

import (
	"github.com/secretflow/kuscia/pkg/ssexporter/parse"
	"strings"
)

func ConvertClusterMetrics(metrics []string) []string {
	var clusterMetrics []string
	clusterAddresses := parse.GetClusterAddress()
	for clusterName := range clusterAddresses {
		for _, metric := range metrics {
			str := "cluster." + clusterName + "." + strings.ToLower(metric)
			clusterMetrics = append(clusterMetrics, str)
		}
	}
	return clusterMetrics
}

func GetEnvoyMetrics() []string {
	var metrics []string
	metrics = append(metrics, "upstream_cx_rx_bytes_total",
		"upstream_cx_total",
		"upstream_rq_total",
		"upstream_cx_tx_bytes_total",
		"health_check.attempt",
		"health_check.failure",
		"upstream_cx_connect_fail",
		"upstream_cx_connect_timeout",
		"upstream_rq_timeout")
	return ConvertClusterMetrics(metrics)
}

func GetEnvoyMetricUrl() string {
	baseUrl := "http://localhost:10000/stats?format=prometheus&&filter="
	metrics := GetEnvoyMetrics()
	filter_regex := "("
	for _, metric := range metrics {
		filter_regex += metric + "|"
	}
	filter_regex = filter_regex[0:len(filter_regex)-1] + ")"
	return baseUrl + filter_regex
}
