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
func GetStatisticFromSs() []map[string]string {
	// execute ss command
	cmd := exec.Command("ss", "-tio4nO")
	output, err := cmd.CombinedOutput()
	if err != nil {
		nlog.Error("Cannot get metrics from ss.", err)
	}
	// parse statistics from ss
	lines := strings.Split(string(output), "\n")
	var tcpStatisticList []map[string]string
	ssMetrics := make(map[string]string)
	for idx, line := range lines {
		if idx < 1 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		ssMetrics = make(map[string]string)
		ssMetrics["rto"] = "0"
		ssMetrics["rtt"] = "0"
		ssMetrics["bytes_sent"] = "0"
		ssMetrics["bytes_received"] = "0"
		ssMetrics["total_connections"] = "1"
		ssMetrics["retrans"] = "0"
		ssMetrics["localAddr"] = fields[3]
		ssMetrics["peerAddr"] = fields[4]
		for idx, field := range fields {
			if idx < 6 {
				continue
			}
			kv := strings.Split(field, ":")
			if len(kv) == 2 {
				switch kv[0] {
				case "rto":
					ssMetrics["rto"] = kv[1]
				case "rtt":
					ssMetrics["rtt"] = strings.Split(kv[1], "/")[0]
				case "bytes_sent":
					ssMetrics["bytes_sent"] = kv[1]
				case "bytes_received":
					ssMetrics["bytes_received"] = kv[1]
				case "retrans":
					ssMetrics["retrans"] = strings.Split(kv[1], "/")[1]
				}
			}
		}
		tcpStatisticList = append(tcpStatisticList, ssMetrics)
	}
	return tcpStatisticList
}

// Filter filter network flows according to given five-tuple rules
func Filter(ssMetrics []map[string]string, srcIP string, dstIP string, srcPort string, dstPort string, protoNum string) []map[string]string {
	// parse the five-tuple rule, wildcard "*" means any field is included
	var results []map[string]string
	for _, metrics := range ssMetrics {
		src := strings.Split(metrics["localAddr"], ":")
		dst := strings.Split(metrics["peerAddr"], ":")
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
func Sum(metrics []map[string]string, key string) float64 {
	sum := 0.0
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Error("fail to parse float", metric[key], "key:", key, "value:", val)
		}
		sum += val
	}
	return sum
}

// Avg an aggregation function to calculate the average of two network metrics
func Avg(metrics []map[string]string, key string) float64 {
	return Sum(metrics, key) / float64(len(metrics))
}

// Max an aggregation function to calculate the maximum of two network metrics
func Max(metrics []map[string]string, key string) float64 {
	max := math.MaxFloat64 * (-1)
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Error("fail to parse float")
		}
		if val > max {
			max = val
		}
	}
	return max
}

// Min an aggregation function to calculate the minimum of two network metrics
func Min(metrics []map[string]string, key string) float64 {
	min := math.MaxFloat64
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Error("fail to parse float")
		}
		if val < min {
			min = val
		}
	}
	return min
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
func AggregateStatistics(localDomainName string, clusterResults map[string]float64, networkResults []map[string]string, aggregationMetrics map[string]string, dstDomain string, MonitorPeriods uint) map[string]float64 {
	if len(networkResults) == 0 {
		return clusterResults
	}
	for metric, aggFunc := range aggregationMetrics {
		if metric != "localAddr" && metric != "peerAddr" {
			if aggFunc == "rate" {
				if metric == "retran_rate" {
					threshold := 0.0
					clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Rate(Sum(networkResults, "retrans")-threshold, Sum(networkResults, "total_connections"))
				}
			} else if aggFunc == "sum" {
				clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Sum(networkResults, metric)
			} else if aggFunc == "avg" {
				clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Avg(networkResults, metric)
			} else if aggFunc == "max" {
				clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Max(networkResults, metric)
			} else if aggFunc == "min" {
				clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Min(networkResults, metric)
			}
			if metric == "bytes_send" || metric == "bytes_received" {
				clusterResults[localDomainName+"."+dstDomain+"."+metric+"."+aggFunc] = Rate(clusterResults[localDomainName+"."+dstDomain+"."+metric], float64(MonitorPeriods))
			}
		}
	}
	return clusterResults
}

// ConvertClusterMetrics convert cluster metrics for prometheus
func ConvertClusterMetrics(ClusterMetrics []string, clusterAddresses map[string][]string) []string {
	var clusterMetrics []string
	for clusterName := range clusterAddresses {
		for _, metric := range ClusterMetrics {
			str := "cluster." + clusterName + "." + strings.ToLower(metric)
			clusterMetrics = append(clusterMetrics, str)
		}
	}
	return clusterMetrics
}

// GetSsMetricResults Get the results of ss statistics after filtering
func GetSsMetricResults(runMode pkgcom.RunModeType, localDomainName string, clusterAddresses map[string][]string, AggregationMetrics map[string]string, MonitorPeriods uint) map[string]float64 {
	// get the statistics from SS
	ssMetrics := GetStatisticFromSs()
	// get the source/destination IP from domain names
	hostName, err := os.Hostname()
	if err != nil {
		nlog.Error("Fail to get the hostname", err)
	}
	sourceIP := parse.GetIpFromDomain(hostName)
	destinationIP := make(map[string][]string)
	for _, endpointAddresses := range clusterAddresses {
		for _, endpointAddress := range endpointAddresses {
			destinationIP[endpointAddress] = parse.GetIpFromDomain(strings.Split(endpointAddress, ":")[0])
		}
	}
	// group metrics by the domain name
	ssResults := make(map[string]float64)
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
		ssResults = AggregateStatistics(localDomainName, ssResults, networkResults, AggregationMetrics, endpointName, MonitorPeriods)
	}

	return ssResults
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
