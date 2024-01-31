// Package metric defines the type of metrics exporting to Prometheus
package promexporter

// NewMetricTypes parse the metric types from a yaml file
func NewMetricTypes() map[string]string {
	MetricTypes := make(map[string]string)
	MetricTypes["rto"] = "Gauge"
	MetricTypes["rtt"] = "Gauge"
	MetricTypes["bytes_sent"] = "Counter"
	MetricTypes["bytes_received"] = "Counter"
	MetricTypes["retran_rate"] = "Gauge"
	MetricTypes["total_connections"] = "Counter"
	MetricTypes["retrans"] = "Counter"
	return MetricTypes
}
