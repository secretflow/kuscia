// Package metric defines the type of metrics exporting to Prometheus
package promexporter

const (
	metricCounter = "Counter"
	metricGauge   = "Gauge"
)

// NewMetricTypes parse the metric types from a yaml file
func NewMetricTypes() map[string]string {
	MetricTypes := make(map[string]string)
	MetricTypes["rto"] = metricGauge
	MetricTypes["rtt"] = metricGauge
	MetricTypes["bytes_sent"] = metricCounter
	MetricTypes["bytes_received"] = metricCounter
	MetricTypes["retran_rate"] = metricGauge
	MetricTypes["total_connections"] = metricCounter
	MetricTypes["retrans"] = metricCounter
	return MetricTypes
}
