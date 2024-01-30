// Package parse configures files and domain files
package parse

// Config define the structure of the configuration file
type MonitorConfig struct {
	SsMetrics  []string
	AggMetrics []string
}

// ReadConfig read the configuration and return each entry
func LoadMetricConfig() ([]string, map[string]string) {
	var config MonitorConfig
	config.SsMetrics = append(config.SsMetrics,
		"rtt",
		"retrans",
		"total_connections",
		"retran_rate")
	aggMetrics := make(map[string]string)
	config.AggMetrics = append(config.AggMetrics, "avg", "sum", "sum", "rate")
	for i, metric := range config.SsMetrics {
		aggMetrics[metric] = config.AggMetrics[i]
	}
	return config.SsMetrics, aggMetrics
}
