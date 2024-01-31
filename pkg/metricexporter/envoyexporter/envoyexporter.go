package envoyexporter

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
	return metrics
}

func GetEnvoyMetricURL() string {
	baseURL := "http://localhost:10000/stats/prometheus?filter="
	metrics := GetEnvoyMetrics()
	filterRegex := "("
	for _, metric := range metrics {
		filterRegex += metric + "|"
	}
	filterRegex = filterRegex[0:len(filterRegex)-1] + ")"
	return baseURL + filterRegex
}
