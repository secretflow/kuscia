package ktmetrics

import (
	"fmt"
	"strings"

	"github.com/secretflow/kuscia/pkg/ktexporter/container_netio"
	"github.com/secretflow/kuscia/pkg/ktexporter/container_stats"
	"github.com/secretflow/kuscia/pkg/ktexporter/fetch_metrics"
	"github.com/secretflow/kuscia/pkg/ktexporter/parse"
	"github.com/secretflow/kuscia/pkg/metricexporter/agg_func"

	"github.com/secretflow/kuscia/pkg/utils/nlog"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
)

type NetStats struct {
	RecvBytes uint64  // Bytes
	XmitBytes uint64  // Bytes
	RecvBW    float64 // bps
	XmitBW    float64 // bps
}

// AggregateStatistics aggregate statistics using an aggregation function
func AggregateStatistics(localDomainName string, clusterResults map[string]float64, networkResults []map[string]string, aggregationMetrics map[string]string, dstDomain string, MonitorPeriods uint) (map[string]float64, error) {
	if len(networkResults) == 0 {
		return clusterResults, nil
	}
	for _, taskid := range networkResults {
		for metric, aggFunc := range aggregationMetrics {
			metricID := localDomainName + ";" + dstDomain + ";" + metric + ";" + aggFunc + ";" + taskid["taskid"]
			nlog.Infof("Generated metricID: %s", metricID)
			nlog.Infof("Generated metricID: %s", metric)
			nlog.Infof("Generated metricID: %s", aggFunc)
			if metric != parse.KtLocalAddr && metric != parse.KtPeerAddr {
				var err error
				if aggFunc == parse.AggSum {
					clusterResults[metricID], err = agg_func.Sum(networkResults, metric)
				} else if aggFunc == parse.AggAvg {
					clusterResults[metricID], err = agg_func.Avg(networkResults, metric)
				} else if aggFunc == parse.AggMax {
					clusterResults[metricID], err = agg_func.Max(networkResults, metric)
				} else if aggFunc == parse.AggMin {
					clusterResults[metricID], err = agg_func.Min(networkResults, metric)
				}
				if err != nil {
					nlog.Warnf("Fail to get clusterResults from aggregation functions, err: %v", err)
					return clusterResults, err
				}
			}
		}
	}
	return clusterResults, nil
}

// Alert an alert function that reports whether a metric value exceeds a given threshold
func Alert(metric float64, threshold float64) bool {
	return metric > threshold
}

// GetKtMetricResults Get the results of kt statistics after filtering
func GetKtMetricResults(runMode pkgcom.RunModeType, localDomainName string, clusterAddresses map[string][]string, AggregationMetrics map[string]string, MonitorPeriods uint) (map[string]float64, error) {
	// Initialize the result map
	ktResults := make(map[string]float64)

	// Iterate over each endpoint address from clusterAddresses (without using IP)
	for _, endpointAddresses := range clusterAddresses {
		for _, endpointAddress := range endpointAddresses {
			// Log some information
			nlog.Info("Processing endpoint address: ", endpointAddress)
			ktMetrics, err := GetStatisticFromKt()
			if err != nil {
				nlog.Warnf("Fail to get statistics from kt, err: %v", err)
				return ktResults, err
			}
			// Get the connection name (e.g., domain or IP)
			endpointName := strings.Split(endpointAddress, ":")[0]

			// Collect all ktMetrics into networkResults (no filtering)
			var networkResults []map[string]string
			networkResults = append(networkResults, ktMetrics...) // Add all ktMetrics data

			// Perform aggregation based on metrics and store results in ktResults
			ktResults, err = AggregateStatistics(localDomainName, ktResults, networkResults, AggregationMetrics, endpointName, MonitorPeriods)
			if err != nil {
				nlog.Warnf("Failed to aggregate statistics for endpoint %s: %v", endpointName, err)
			}

		}
	}
	// Return the aggregated ktResults
	return ktResults, nil
}

func GetStatisticFromKt() ([]map[string]string, error) {
	var preRecvBytes, preXmitBytes uint64
	var tcpStatisticList []map[string]string
	timeWindow := 1.0
	taskToPID, err := fetch_metrics.GetKusciaTaskPID()
	if err != nil {
		nlog.Error("Fail to get container PIDs", err)
		return nil, err
	}

	taskIDToContainerID, err := fetch_metrics.GetTaskIDToContainerID()
	if err != nil {
		nlog.Error("Fail to get container ID", err)
		return nil, err
	}

	for kusciaTaskID, containerPID := range taskToPID {
		containerID, exists := taskIDToContainerID[kusciaTaskID]
		if !exists || containerID == "" {
			// Skip this task if no valid CID is found
			continue
		}

		recvBytes, xmitBytes, err := container_netio.GetContainerNetIOFromProc("eth0", containerPID)
		if err != nil {
			nlog.Warnf("Fail to get container network IO from proc")
			continue
		}

		recvBW, xmitBW, err := container_netio.GetContainerBandwidth(recvBytes, preRecvBytes, xmitBytes, preXmitBytes, timeWindow)
		if err != nil {
			nlog.Warnf("Fail to get the network bandwidth of containers")
			continue
		}
		preRecvBytes = recvBytes
		preXmitBytes = xmitBytes
		ktMetrics := make(map[string]string)
		ktMetrics[parse.MetricRecvBytes] = fmt.Sprintf("%d", recvBytes)
		ktMetrics[parse.MetricXmitBytes] = fmt.Sprintf("%d", xmitBytes)
		ktMetrics[parse.MetricRecvBw] = fmt.Sprintf("%.2f", recvBW)
		ktMetrics[parse.MetricXmitBw] = fmt.Sprintf("%.2f", xmitBW)
		ktMetrics["taskid"] = kusciaTaskID
		containerStats, err := container_stats.GetContainerStats()
		if err != nil {
			nlog.Warn("Fail to get the stats of containers")
			continue
		}
		ktMetrics[parse.MetricCPUPercentage] = containerStats[containerID].CPUPercentage
		ktMetrics[parse.MetricDisk] = containerStats[containerID].Disk
		ktMetrics[parse.MetricInodes] = containerStats[containerID].Inodes
		ktMetrics[parse.MetricMemory] = containerStats[containerID].Memory

		cpuUsage, err := fetch_metrics.GetTotalCPUUsageStats(containerID)
		if err != nil {
			nlog.Warn("Fail to get the total CPU usage stats")
			continue
		}
		ktMetrics[parse.MetricCPUUsage] = fmt.Sprintf("%d", cpuUsage)

		virtualMemory, physicalMemory, err := fetch_metrics.GetMaxMemoryUsageStats(containerPID, containerID)
		if err != nil {
			nlog.Warn("Fail to get the total memory stats")
			continue
		}
		ktMetrics[parse.MetricVirtualMemory] = fmt.Sprintf("%d", virtualMemory)
		ktMetrics[parse.MetricPhysicalMemory] = fmt.Sprintf("%d", physicalMemory)

		tcpStatisticList = append(tcpStatisticList, ktMetrics)
	}

	return tcpStatisticList, nil
}
