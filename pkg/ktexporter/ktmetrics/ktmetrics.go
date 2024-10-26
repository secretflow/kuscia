package ktmetrics

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"os"

	"github.com/secretflow/kuscia/pkg/ktexporter/parse"

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
		fmt.Println("failed to execute crictl ps.", err)
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	if len(lines) < 2 {
		fmt.Println("unexpected output format from crictl ps", err)
		return nil, err
	}

	taskIDToContainerID := make(map[string]string)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			fmt.Printf("unexpected output format for line: %s", line)
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
		fmt.Println("failed to execute crictl stats", err)
		//nlog.Warn("failed to execute crictl stats", err)
		return nil, err
	}

	// Parse the output
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 2 {
		fmt.Println("unexpected output format from crictl stats")
		//nlog.Warn("unexpected output format from crictl stats")
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
			fmt.Printf("unexpected output format for line: %s", line)
			//nlog.Warn("unexpected output format for line: %s", line)
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

// GetIOStats retrieves I/O statistics (read_bytes, write_bytes) for a given PID
func GetTotalIOStats(pid string) (uint64, uint64, error) {
	// Read /proc/[pid]/io
	ioFile := filepath.Join("/proc", pid, "io")
	ioData, err := ioutil.ReadFile(ioFile)
	if err != nil {
		nlog.Warn("failed to read /proc/[pid]/io", err)
		return 0, 0, err
	}

	var readBytes, writeBytes uint64

	// Parse the file contents to find read_bytes and write_bytes
	lines := strings.Split(string(ioData), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "read_bytes:") {
			parts := strings.Fields(line)
			readBytes, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				nlog.Warn("failed to parse read_bytes", err)
				return 0, 0, err
			}
		}
		if strings.HasPrefix(line, "write_bytes:") {
			parts := strings.Fields(line)
			writeBytes, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				nlog.Warn("failed to parse write_bytes", err)
				return 0, 0, err
			}
		}
	}

	return readBytes, writeBytes, nil
}

func GetContainerNetIOFromProc(defaultIface, pid string) (recvBytes, xmitBytes uint64, err error) {
	netDevPath := fmt.Sprintf("/proc/%s/net/dev", pid)
	data, err := ioutil.ReadFile(netDevPath)
	if err != nil {
		//nlog.Warn("Fail to read the path", netDevPath)
		return recvBytes, xmitBytes, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 {
		//nlog.Error("unexpected format in ", netDevPath)
		fmt.Println("unexpected format in ", netDevPath)
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
		//nlog.Error("Error converting string to uint64:", err)
		fmt.Println("Error converting string to uint64:", err)
		return recvBytes, xmitBytes, err
	}
	xmitBytes, err = strconv.ParseUint(xmitByteStr, 10, 64)
	if err != nil {
		//nlog.Error("Error converting string to uint64:", err)
		fmt.Println("Error converting string to uint64:", err)
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
	}
	return clusterResults, nil
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
		nlog.Warnf("Fail to get the sum of kt metrics, err: %v", err)
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

func GetMetricChange(lastMetricValues map[string]float64, currentMetricValues map[string]float64) (map[string]float64, map[string]float64) {
	for metric, value := range currentMetricValues {
		nlog.Info("3333366666")
		currentMetricValues[metric] = currentMetricValues[metric] - lastMetricValues[metric]
		if currentMetricValues[metric] < 0 {
			currentMetricValues[metric] = 0
		}
		lastMetricValues[metric] = value

	}
	return lastMetricValues, currentMetricValues
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
		ktMetrics[parse.MetricCPUPer] = containerStats[containerID].CPUPercentage
		ktMetrics[parse.MetricDisk] = containerStats[containerID].Disk
		ktMetrics[parse.MetricInodes] = containerStats[containerID].Inodes
		ktMetrics[parse.MetricMemory] = containerStats[containerID].Memory

		cpuUsage, err := GetTotalCPUUsageStats(containerID)
		if err != nil {
			nlog.Warn("Fail to get the total CPU usage stats")
			continue
		}
		ktMetrics[parse.MetricCPUUs] = fmt.Sprintf("%d", cpuUsage)

		virtualMemory, physicalMemory, err := GetMaxMemoryUsageStats(containerPID, containerID)
		if err != nil {
			nlog.Warn("Fail to get the total memory stats")
			continue
		}
		ktMetrics[parse.MetricVtMemory] = fmt.Sprintf("%d", virtualMemory)
		ktMetrics[parse.MetricPsMemory] = fmt.Sprintf("%d", physicalMemory)

		readBytes, writeBytes, err := GetTotalIOStats(containerPID)
		if err != nil {
			nlog.Warn("Fail to get the total IO stats")
			continue
		}
		ktMetrics[parse.MetricReadBytes] = fmt.Sprintf("%d", readBytes)
		ktMetrics[parse.MetricWriteBytes] = fmt.Sprintf("%d", writeBytes)

		tcpStatisticList = append(tcpStatisticList, ktMetrics)
	}

	return tcpStatisticList, nil
}
