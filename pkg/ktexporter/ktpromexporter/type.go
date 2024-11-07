// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package metric defines the type of metrics exporting to Prometheus
package ktpromexporter

const (
	metricCounter = "Counter"
	metricGauge   = "Gauge"
)

// NewMetricTypes_kt parse the metric types from a yaml file

func NewMetricTypes_kt() map[string]string {
	MetricTypes := make(map[string]string)
	MetricTypes["recvbw"] = metricGauge
	MetricTypes["xmitbw"] = metricGauge
	MetricTypes["recvbytes"] = metricGauge
	MetricTypes["xmitbytes"] = metricGauge
	MetricTypes["cpu_percentage"] = metricGauge
	MetricTypes["virtual_memory"] = metricGauge
	MetricTypes["physical_memory"] = metricGauge
	MetricTypes["total_cpu_time_ns"] = metricGauge
	MetricTypes["memory_usage"] = metricGauge
	MetricTypes["disk_io"] = metricGauge
	MetricTypes["inodes"] = metricGauge
	return MetricTypes
}
