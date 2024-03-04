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
