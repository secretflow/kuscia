package ktmetrics

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"os"

	"github.com/secretflow/kuscia/pkg/ktexporter/parse"
	"github.com/secretflow/kuscia/pkg/metricexporter/agg_func"

	jsoniter "github.com/json-iterator/go"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
)

// ContainerConfig represents the config.json structure.
type ContainerConfig struct {
	Hostname string `json:"hostname"`
}

type NetStats struct {
	RecvBytes uint64  // Bytes
	XmitBytes uint64  // Bytes
	RecvBW    float64 // bps
	XmitBW    float64 // bps
}

type KusicaTaskStats struct {
	CtrStats ContainerStats
	NetIO    NetStats
}

func GetKusciaTaskPID() (map[string]string, error) {
	const containerdDir = "/home/kuscia/containerd/run/io.containerd.runtime.v2.task/k8s.io/"
	taskIDToPID := make(map[string]string)
	containerFiles, err := ioutil.ReadDir(containerdDir)
	if err != nil {
		nlog.Info("Error reading directory:", err)
		return taskIDToPID, err
	}
	for _, containerFile := range containerFiles {
		if !containerFile.IsDir() {
			continue
		}
		containerDir := filepath.Join(containerdDir, containerFile.Name())
		// Read init.pid
		pidFile := filepath.Join(containerDir, "init.pid")
		pidData, err := ioutil.ReadFile(pidFile)
		if err != nil {
			nlog.Info("Error reading pid containerFile:", err)
			return taskIDToPID, err
		}

		// Read config.json
		configFile := filepath.Join(containerDir, "config.json")
		configData, err := ioutil.ReadFile(configFile)
		if err != nil {
			nlog.Info("Error reading config containerFile:", err)
			return taskIDToPID, err
		}

		var config ContainerConfig
		err = jsoniter.Unmarshal(configData, &config)
		if err != nil {
			nlog.Info("Error parsing config containerFile:", err)
			return taskIDToPID, err
		}
		if config.Hostname != "" {
			taskIDToPID[config.Hostname] = string(pidData)
		}
	}

	return taskIDToPID, nil
}

// GetContainerMappings fetches the container info using crictl ps command
func GetTaskIDToContainerID() (map[string]string, error) {
	cmd := exec.Command("crictl", "ps")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		nlog.Error("failed to execute crictl ps.", err)
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	if len(lines) < 2 {
		nlog.Warn("unexpected output format from crictl ps", err)
		return nil, err
	}

	taskIDToContainerID := make(map[string]string)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			nlog.Warn("unexpected output format for line: %s", line)
			return nil, err
		}
		state := fields[5] // 通常，crictl ps 的第四个字段为状态(State)
		if state != "Running" {
			nlog.Infof("state is %s", state)
			continue
		}
		containerID := fields[0]
		kusciaTaskID := fields[len(fields)-1]
		taskIDToContainerID[kusciaTaskID] = containerID
	}

	return taskIDToContainerID, nil
}

// ContainerStats holds the stats information for a container
type ContainerStats struct {
	CPUPercentage string
	Memory        string
	Disk          string
	Inodes        string
}

// GetContainerStats fetches the container stats using crictl stats command
func GetContainerStats() (map[string]ContainerStats, error) {
	// Execute the crictl stats command
	cmd := exec.Command("crictl", "stats")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		nlog.Warn("failed to execute crictl stats", err)
		return nil, err
	}

	// Parse the output
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 2 {
		nlog.Warn("unexpected output format from crictl stats")
		return nil, err
	}

	// Create a map to hold the stats for each container
	statsMap := make(map[string]ContainerStats)

	// Skip the header line and iterate over the output lines
	for _, line := range lines[1:] {
		// Skip empty lines
		if line == "" {
			continue
		}

		// Split the line by whitespace
		fields := strings.Fields(line)
		if len(fields) < 5 {
			nlog.Warn("unexpected output format for line: %s", line)
			return nil, err
		}

		// Extract the stats information
		containerID := fields[0]
		stats := ContainerStats{
			CPUPercentage: fields[1],
			Memory:        fields[2],
			Disk:          fields[3],
			Inodes:        fields[4],
		}
		// Add the stats to the map
		statsMap[containerID] = stats
	}

	return statsMap, nil
}

// GetMemoryUsageStats retrieves memory usage statistics (virtual, physical, swap) for a given container
func GetMaxMemoryUsageStats(pid string, cidPrefix string) (uint64, uint64, error) {
	// Find the full CID based on the prefix
	cid, err := findCIDByPrefix(cidPrefix)
	if err != nil {
		nlog.Warn("failed to find full CID by prefix", err)
		return 0, 0, err
	}

	// Get Virtual Memory (VmPeak)
	virtualMemory, err := getVirtualMemoryUsage(pid)
	if err != nil {
		return 0, 0, err
	}

	// Get Physical and Swap Memory (max usage in bytes)
	physicalMemory, err := getPhysicalMemoryUsage(cid)
	if err != nil {
		return 0, 0, err
	}

	return virtualMemory, physicalMemory, nil
}

// getVirtualMemoryUsage retrieves the peak virtual memory usage for a given PID
func getVirtualMemoryUsage(pid string) (uint64, error) {
	// Read /proc/[pid]/status
	statusFile := filepath.Join("/proc", pid, "status")
	statusData, err := ioutil.ReadFile(statusFile)
	if err != nil {
		nlog.Warn("failed to read /proc/[pid]/status", err)
		return 0, err
	}

	// Parse VmPeak from status
	lines := strings.Split(string(statusData), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmPeak:") {
			parts := strings.Fields(line)
			if len(parts) == 3 {
				vmPeak, err := strconv.ParseUint(parts[1], 10, 64)
				if err != nil {
					nlog.Warn("failed to parse VmPeak value", err)
					return 0, err
				}
				return vmPeak * 1024, nil // Convert to bytes
			}
		}
	}

	nlog.Warn("VmPeak not found in /proc/[pid]/status")
	return 0, fmt.Errorf("VmPeak not found in /proc/[pid]/status")
}

// getPhysicalMemoryUsage retrieves the peak physical memory usage for a cgroup
func getPhysicalMemoryUsage(cid string) (uint64, error) {
	// Read /sys/fs/cgroup/memory/k8s.io/[cid]/memory.max_usage_in_bytes
	memMaxUsageFile := filepath.Join("/sys/fs/cgroup/memory/k8s.io", cid, "memory.max_usage_in_bytes")
	memMaxUsageData, err := ioutil.ReadFile(memMaxUsageFile)
	if err != nil {
		nlog.Warn("failed to read memory.max_usage_in_bytes", err)
		return 0, err
	}

	// Parse the max usage in bytes
	physicalMemory, err := strconv.ParseUint(strings.TrimSpace(string(memMaxUsageData)), 10, 64)
	if err != nil {
		nlog.Warn("failed to parse memory.max_usage_in_bytes", err)
		return 0, err
	}

	return physicalMemory, nil
}

func GetContainerNetIOFromProc(defaultIface, pid string) (recvBytes, xmitBytes uint64, err error) {
	netDevPath := fmt.Sprintf("/proc/%s/net/dev", pid)
	data, err := ioutil.ReadFile(netDevPath)
	if err != nil {
		nlog.Warn("Fail to read the path", netDevPath)
		return recvBytes, xmitBytes, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 {
		nlog.Error("unexpected format in ", netDevPath)
		return recvBytes, xmitBytes, err
	}
	recvByteStr := ""
	xmitByteStr := ""
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		iface := strings.Trim(fields[0], ":")
		if iface == defaultIface {
			recvByteStr = fields[1]
			xmitByteStr = fields[9]
		}
	}
	if recvByteStr == "" {
		recvByteStr = "0"
	}
	if xmitByteStr == "" {
		xmitByteStr = "0"
	}
	recvBytes, err = strconv.ParseUint(recvByteStr, 10, 64)
	if err != nil {
		nlog.Error("Error converting string to uint64:", err)
		return recvBytes, xmitBytes, err
	}
	xmitBytes, err = strconv.ParseUint(xmitByteStr, 10, 64)
	if err != nil {
		nlog.Error("Error converting string to uint64:", err)
		return recvBytes, xmitBytes, err
	}

	return recvBytes, xmitBytes, nil
}

func GetContainerBandwidth(curRecvBytes, preRecvBytes, curXmitBytes, preXmitBytes uint64, timeWindow float64) (recvBandwidth, xmitBandwidth float64, err error) {
	recvBytesDiff := float64(curRecvBytes) - float64(preRecvBytes)
	xmitBytesDiff := float64(curXmitBytes) - float64(preXmitBytes)

	recvBandwidth = (recvBytesDiff * 8) / timeWindow
	xmitBandwidth = (xmitBytesDiff * 8) / timeWindow

	return recvBandwidth, xmitBandwidth, nil
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

func findCIDByPrefix(prefix string) (string, error) {
	cgroupDir := "/sys/fs/cgroup/cpu/k8s.io/"
	files, err := ioutil.ReadDir(cgroupDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) {
			return file.Name(), nil
		}
	}

	return "", os.ErrNotExist
}

func GetTotalCPUUsageStats(cidPrefix string) (uint64, error) {
	//var stats CPUUsageStats

	// Find the full CID based on the prefix
	cid, err := findCIDByPrefix(cidPrefix)
	if err != nil {
		nlog.Warn("failed to find full CID by prefix", err)
		return 0, err
	}

	// Read /sys/fs/cgroup/cpu/k8s.io/[cid]/cpuacct.usage
	cgroupUsageFile := filepath.Join("/sys/fs/cgroup/cpu/k8s.io", cid, "cpuacct.usage")
	cgroupUsageData, err := ioutil.ReadFile(cgroupUsageFile)
	if err != nil {
		nlog.Warn("failed to read cpuacct.usage", err)
		return 0, err
	}

	// Parse cpuacct.usage
	cpuUsage, err := strconv.ParseUint(strings.TrimSpace(string(cgroupUsageData)), 10, 64)
	if err != nil {
		nlog.Warn("failed to parse cpuacct.usage", err)
		return 0, err
	}

	return cpuUsage, nil
}

func GetStatisticFromKt() ([]map[string]string, error) {
	var preRecvBytes, preXmitBytes uint64
	var tcpStatisticList []map[string]string
	timeWindow := 1.0
	taskToPID, err := GetKusciaTaskPID()
	if err != nil {
		nlog.Error("Fail to get container PIDs", err)
	}

	taskIDToContainerID, err := GetTaskIDToContainerID()
	if err != nil {
		nlog.Error("Fail to get container ID", err)
	}

	for kusciaTaskID, containerPID := range taskToPID {
		containerID, exists := taskIDToContainerID[kusciaTaskID]
		if !exists || containerID == "" {
			// Skip this task if no valid CID is found
			continue
		}

		recvBytes, xmitBytes, err := GetContainerNetIOFromProc("eth0", containerPID)
		if err != nil {
			nlog.Warn("Fail to get container network IO from proc")
			continue
		}

		recvBW, xmitBW, err := GetContainerBandwidth(recvBytes, preRecvBytes, xmitBytes, preXmitBytes, timeWindow)
		if err != nil {
			nlog.Warn("Fail to get the network bandwidth of containers")
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
		containerStats, err := GetContainerStats()
		if err != nil {
			nlog.Warn("Fail to get the stats of containers")
			continue
		}
		ktMetrics[parse.MetricCPUPercentage] = containerStats[containerID].CPUPercentage
		ktMetrics[parse.MetricDisk] = containerStats[containerID].Disk
		ktMetrics[parse.MetricInodes] = containerStats[containerID].Inodes
		ktMetrics[parse.MetricMemory] = containerStats[containerID].Memory

		cpuUsage, err := GetTotalCPUUsageStats(containerID)
		if err != nil {
			nlog.Warn("Fail to get the total CPU usage stats")
			continue
		}
		ktMetrics[parse.MetricCPUUsage] = fmt.Sprintf("%d", cpuUsage)

		virtualMemory, physicalMemory, err := GetMaxMemoryUsageStats(containerPID, containerID)
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
