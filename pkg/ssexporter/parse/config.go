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

// Package parse configures files and domain files
package parse

// Config define the structure of the configuration file
type MonitorConfig struct {
	SsMetrics  []string
	AggMetrics []string
}

const (
	MetricRTT              = "rtt"
	MetricRetrans          = "retrans"
	MetricTotalConnections = "total_connections"
	MetricRetranRate       = "retran_rate"
	MetricRto              = "rto"
	MetricByteSent         = "bytes_sent"
	MetricBytesReceived    = "bytes_received"

	SsLocalAddr = "localAddr"
	SsPeerAddr  = "peerAddr"

	AggSum   = "sum"
	AggAvg   = "avg"
	AggMax   = "max"
	AggMin   = "min"
	AggAlert = "alert"
	AggRate  = "rate"
)

// ReadConfig read the configuration and return each entry
func LoadMetricConfig() ([]string, map[string]string) {
	var config MonitorConfig
	config.SsMetrics = append(config.SsMetrics,
		MetricRTT,
		MetricRetrans,
		MetricTotalConnections,
		MetricRetranRate)
	aggMetrics := make(map[string]string)
	config.AggMetrics = append(config.AggMetrics, AggAvg, AggSum, AggSum, AggRate)
	for i, metric := range config.SsMetrics {
		aggMetrics[metric] = config.AggMetrics[i]
	}
	return config.SsMetrics, aggMetrics
}
