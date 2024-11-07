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
	KtMetrics  []string
	AggMetrics []string
}

const (
	MetricRecvBytes      = "recvbytes"
	MetricXmitBytes      = "xmitbytes"
	MetricRecvBw         = "recvbw"
	MetricXmitBw         = "xmitbw"
	MetricCPUPercentage  = "cpu_percentage"
	MetricCPUUsage       = "total_cpu_time_ns"
	MetricVirtualMemory  = "virtual_memory"
	MetricPhysicalMemory = "physical_memory"
	MetricMemory         = "memory_usage"
	MetricDisk           = "disk_io"
	MetricInodes         = "inodes"
	KtLocalAddr          = "localAddr"
	KtPeerAddr           = "peerAddr"

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
	config.KtMetrics = append(config.KtMetrics,
		MetricRecvBytes,
		MetricXmitBytes,
		MetricRecvBw,
		MetricXmitBw,
		MetricCPUPercentage,
		MetricCPUUsage,
		MetricMemory,
		MetricDisk,
		MetricInodes,
		MetricVirtualMemory,
		MetricPhysicalMemory)
	aggMetrics := make(map[string]string)
	config.AggMetrics = append(config.AggMetrics, AggSum, AggSum, AggSum, AggSum, AggAvg, AggAvg, AggAvg, AggSum, AggSum, AggSum, AggSum)
	for i, metric := range config.KtMetrics {
		aggMetrics[metric] = config.AggMetrics[i]
	}
	return config.KtMetrics, aggMetrics
}
