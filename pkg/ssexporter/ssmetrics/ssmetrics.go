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

// Package ssmetrics collect metrics from ss
package ssmetrics

import (
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/ssexporter/parse"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// GetStatisticFromSs get the statistics of network flows from ss
func GetStatisticFromSs() ([]map[string]string, error) {
	// execute ss command
	// -H --no-header Suppress header line
	cmd := exec.Command("ss", "-tio4nH")
	output, err := cmd.CombinedOutput()

	if err != nil {
		nlog.Warnf("Cannot get metrics from ss, error: %v", err)
		return nil, err
	}
	// parse statistics from ss
	lines := strings.Split(string(output), "\n")

	var tcpStatisticList []map[string]string
	// Combine every two lines and parse them
	for i := 0; i+1 < len(lines); i += 2 {
		combinedLine := lines[i] + lines[i+1]
		// Parse the combined line
		if ssMetrics := parseCombinedLine(combinedLine); ssMetrics != nil {
			tcpStatisticList = append(tcpStatisticList, ssMetrics)
		}
	}
	return tcpStatisticList, nil
}

func parseCombinedLine(line string) map[string]string {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return nil
	}
	ssMetrics := make(map[string]string)
	ssMetrics[parse.MetricRto] = "0"
	ssMetrics[parse.MetricRTT] = "0"
	ssMetrics[parse.MetricByteSent] = "0"
	ssMetrics[parse.MetricBytesReceived] = "0"
	ssMetrics[parse.MetricTotalConnections] = "1"
	ssMetrics[parse.MetricRetrans] = "0"
	ssMetrics[parse.SsLocalAddr] = fields[3]
	ssMetrics[parse.SsPeerAddr] = fields[4]

	for idx := 6; idx < len(fields); idx++ {
		field := fields[idx]
		kv := strings.Split(field, ":")
		if len(kv) == 2 {
			switch kv[0] {
			case parse.MetricRto:
				ssMetrics[parse.MetricRto] = kv[1]
			case parse.MetricRTT:
				ssMetrics[parse.MetricRTT] = strings.Split(kv[1], "/")[0]
			case parse.MetricByteSent:
				ssMetrics[parse.MetricByteSent] = kv[1]
			case parse.MetricBytesReceived:
				ssMetrics[parse.MetricBytesReceived] = kv[1]
			case parse.MetricRetrans:
				ssMetrics[parse.MetricRetrans] = strings.Split(kv[1], "/")[1]
			}
		}
	}
	return ssMetrics
}

// Filter filter network flows according to given five-tuple rules
func Filter(ssMetrics []map[string]string, srcIP string, dstIP string, srcPort string, dstPort string, protoNum string) []map[string]string {
	// parse the five-tuple rule, wildcard "*" means any field is included
	var results []map[string]string
	for _, metrics := range ssMetrics {
		src := strings.Split(metrics[parse.SsLocalAddr], ":")
		dst := strings.Split(metrics[parse.SsPeerAddr], ":")
		if (srcIP != "*") && (src[0] != srcIP) {
			continue
		}
		if (dstIP != "*") && (dst[0] != dstIP) {
			continue
		}
		if (srcPort != "*") && (src[1] != srcPort) {
			continue
		}
		if (dstPort != "*") && (dst[1] != dstPort) {
			continue
		}
		if protoNum != "*" {
			continue
		}
		results = append(results, metrics)
	}
	return results
}

// Sum an aggregation function to sum up two network metrics
func Sum(metrics []map[string]string, key string) (float64, error) {
	sum := 0.0
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warnf("fail to parse float: %s, key: %s, value: %f", metric[key], key, val)
			return sum, err
		}
		sum += val
	}
	return sum, nil
}

// Avg an aggregation function to calculate the average of two network metrics
func Avg(metrics []map[string]string, key string) (float64, error) {
	sum, err := Sum(metrics, key)
	if err != nil {
		nlog.Warnf("Fail to get the sum of ss metrics, err: %v", err)
		return sum, err
	}
	return sum / float64(len(metrics)), nil
}

// Max an aggregation function to calculate the maximum of two network metrics
func Max(metrics []map[string]string, key string) (float64, error) {
	max := math.MaxFloat64 * (-1)
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warn("fail to parse float")
			return max, err
		}
		if val > max {
			max = val
		}
	}
	return max, nil
}

// Min an aggregation function to calculate the minimum of two network metrics
func Min(metrics []map[string]string, key string) (float64, error) {
	min := math.MaxFloat64
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warn("fail to parse float")
			return min, err
		}
		if val < min {
			min = val
		}
	}
	return min, nil
}

// Rate an aggregation function to calculate the rate of a network metric between to metrics
func Rate(metric1 float64, metric2 float64) float64 {
	if metric2 == 0.0 {
		return 0
	}
	return metric1 / metric2
}

// Alert an alert function that reports whether a metric value exceeds a given threshold
func Alert(metric float64, threshold float64) bool {
	return metric > threshold
}

// AggregateStatistics aggregate statistics using an aggregation function
func AggregateStatistics(localDomainName string, clusterResults map[string]float64, networkResults []map[string]string, aggregationMetrics map[string]string, dstDomain string, MonitorPeriods uint) (map[string]float64, error) {
	if len(networkResults) == 0 {
		return clusterResults, nil
	}
	for metric, aggFunc := range aggregationMetrics {
		metricID := localDomainName + ";" + dstDomain + ";" + metric + ";" + aggFunc
		if metric != parse.SsLocalAddr && metric != parse.SsPeerAddr {
			var err error
			if aggFunc == parse.AggRate {
				if metric == parse.MetricRetranRate {
					threshold := 0.0
					retranSum, err := Sum(networkResults, parse.MetricRetrans)
					connectSum, err := Sum(networkResults, parse.MetricTotalConnections)
					if err != nil {
						return clusterResults, err
					}
					clusterResults[metricID] = Rate(retranSum-threshold, connectSum)
				}
			} else if aggFunc == parse.AggSum {
				clusterResults[metricID], err = Sum(networkResults, metric)
			} else if aggFunc == parse.AggAvg {
				clusterResults[metricID], err = Avg(networkResults, metric)
			} else if aggFunc == parse.AggMax {
				clusterResults[metricID], err = Max(networkResults, metric)
			} else if aggFunc == parse.AggMin {
				clusterResults[metricID], err = Min(networkResults, metric)
			}
			if err != nil {
				nlog.Warnf("Fail to get clusterResults from aggregation functions, err: %v", err)
				return clusterResults, err
			}
			if metric == parse.MetricByteSent || metric == parse.MetricBytesReceived {
				clusterResults[metricID] = Rate(clusterResults[metricID], float64(MonitorPeriods))
			}
		}
	}
	return clusterResults, nil
}

// GetSsMetricResults Get the results of ss statistics after filtering
func GetSsMetricResults(runMode pkgcom.RunModeType, localDomainName string, clusterAddresses map[string][]string, AggregationMetrics map[string]string, MonitorPeriods uint) (map[string]float64, error) {
	// get the statistics from SS
	ssResults := make(map[string]float64)
	ssMetrics, err := GetStatisticFromSs()
	if err != nil {
		nlog.Warnf("Fail to get statistics from ss, err: %v", err)
		return ssResults, err
	}
	// get the source/destination IP from domain names
	hostName, err := os.Hostname()
	if err != nil {
		nlog.Warnf("Fail to get the hostname, err: %v", err)
		return ssResults, err
	}
	sourceIP := parse.GetIPFromDomain(hostName)
	destinationIP := make(map[string][]string)
	for _, endpointAddresses := range clusterAddresses {
		for _, endpointAddress := range endpointAddresses {
			destinationIP[endpointAddress] = parse.GetIPFromDomain(strings.Split(endpointAddress, ":")[0])
		}
	}
	// group metrics by the domain name
	for dstAddr := range destinationIP {
		// get the connection name
		endpointName := strings.Split(dstAddr, ":")[0]
		var networkResults []map[string]string
		for _, srcIP := range sourceIP {
			for _, dstIP := range destinationIP[dstAddr] {
				networkResult := Filter(ssMetrics, srcIP, dstIP, "*", "*", "*")
				networkResults = append(networkResults, networkResult...)
			}
		}
		ssResults, err = AggregateStatistics(localDomainName, ssResults, networkResults, AggregationMetrics, endpointName, MonitorPeriods)
	}

	return ssResults, nil
}

// GetMetricChange get the change values of metrics
func GetMetricChange(lastMetricValues map[string]float64, currentMetricValues map[string]float64) (map[string]float64, map[string]float64) {
	for metric, value := range currentMetricValues {
		currentMetricValues[metric] = currentMetricValues[metric] - lastMetricValues[metric]
		if currentMetricValues[metric] < 0 {
			currentMetricValues[metric] = 0
		}
		lastMetricValues[metric] = value

	}
	return lastMetricValues, currentMetricValues
}
