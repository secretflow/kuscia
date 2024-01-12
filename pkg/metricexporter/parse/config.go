// Package parse configures files and domain files
package parse

// Config define the structure of the configuration file
type MonitorConfig struct {
	NetMetrics []string
	AggMetrics []string
	CluMetrics []string
}

// ReadConfig read the configuration and return each entry
func LoadMetricConfig() ([]string, map[string]string, []string) {
	var config MonitorConfig
	config.NetMetrics = append(config.NetMetrics,
		"rtt",
		"retrans",
		"total_connections",
		"retran_rate")
	config.CluMetrics = append(config.CluMetrics, "upstream_cx_rx_bytes_total",
		"upstream_cx_total",
		"upstream_rq_total",
		"upstream_cx_tx_bytes_total",
		"health_check.attempt",
		"health_check.failure",
		"upstream_cx_connect_fail",
		"upstream_cx_connect_timeout",
		"upstream_rq_timeout")
	aggMetrics := make(map[string]string)
	config.AggMetrics = append(config.AggMetrics, "avg", "avg", "sum", "rate")
	for i, metric := range config.NetMetrics {
		aggMetrics[metric] = config.AggMetrics[i]
	}
	return config.NetMetrics, aggMetrics, config.CluMetrics
}
