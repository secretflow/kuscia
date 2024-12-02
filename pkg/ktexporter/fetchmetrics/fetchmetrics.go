package fetchmetrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "github.com/containerd/cgroups/stats/v1"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers/core/v1"
)

type ContainerConfig struct {
	Hostname string `json:"hostname"`
}

func GetKusciaTaskPID() (map[string]string, error) {
	const containerdDir = "/home/kuscia/containerd/run/io.containerd.runtime.v2.task/k8s.io/"
	taskIDToPID := make(map[string]string)
	containerFiles, err := os.ReadDir(containerdDir)
	if err != nil {
		nlog.Error("Error reading directory:", err)
		return taskIDToPID, err
	}
	for _, containerFile := range containerFiles {
		if !containerFile.IsDir() {
			continue
		}
		containerDir := filepath.Join(containerdDir, containerFile.Name())
		// Read init.pid
		pidFile := filepath.Join(containerDir, "init.pid")
		pidData, err := os.ReadFile(pidFile)
		if err != nil {
			nlog.Info("Error reading pid containerFile:", err)
			return taskIDToPID, err
		}

		// Read config.json
		configFile := filepath.Join(containerDir, "config.json")
		configData, err := os.ReadFile(configFile)
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

func GetTaskIDToContainerID(podLister listers.PodLister) (map[string]string, error) {
	var pods []*corev1.Pod
	pods, err := podLister.List(labels.Everything()) // 这里不做任何标签筛选
	if err != nil {
		return nil, err
	}
	taskIDToContainerID := make(map[string]string)

	for _, pod := range pods {
		annotations := pod.Annotations
		if annotations == nil || annotations[common.TaskIDAnnotationKey] == "" {
			continue
		}
		taskID := annotations[common.TaskIDAnnotationKey]
		for _, container := range pod.Status.ContainerStatuses {
			containerID := container.ContainerID
			taskIDToContainerID[taskID] = containerID
		}
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
func GetContainerStats(podLister listers.PodLister) (map[string]ContainerStats, error) {

	var pods []*corev1.Pod
	pods, err := podLister.List(labels.Everything()) // 这里不做任何标签筛选
	if err != nil {
		return nil, err
	}
	statsMap := make(map[string]ContainerStats)

	for _, pod := range pods {

		for _, container := range pod.Status.ContainerStatuses {
			containerID := container.ContainerID

			client, err := containerd.New("/run/containerd/containerd.sock")
			if err != nil {
				nlog.Fatal(err)
			}
			defer client.Close()

			ctx := namespaces.WithNamespace(context.Background(), "default")

			container, err := client.LoadContainer(ctx, containerID)
			if err != nil {
				nlog.Fatalf("failed to load container: %v", err)
			}

			task, err := container.Task(ctx, cio.Load)
			if err != nil {
				nlog.Fatalf("failed to load task for container: %v", err)
			}

			metrics, err := task.Metrics(ctx)
			if err != nil {
				nlog.Fatalf("failed to get metrics: %v", err)
			}

			newAny := &types.Any{
				TypeUrl: metrics.Data.GetTypeUrl(),
				Value:   metrics.Data.GetValue(),
			}

			data, err := typeurl.UnmarshalAny(newAny)
			if err != nil {
				nlog.Fatalf("failed to unmarshal metrics data: %v", err)
			}

			ans := data.(*v1.Metrics)
			usageStr1 := strconv.FormatUint(ans.CPU.Usage.Total, 10)
			usageStr2 := strconv.FormatUint(ans.Memory.Usage.Usage, 10)

			stats := ContainerStats{
				CPUPercentage: usageStr1,
				Memory:        usageStr2,
				Disk:          "",
				Inodes:        "",
			}

			statsMap[containerID] = stats
		}
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
	statusData, err := os.ReadFile(statusFile)
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
	memMaxUsageData, err := os.ReadFile(memMaxUsageFile)
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
	data, err := os.ReadFile(netDevPath)
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

func findCIDByPrefix(prefix string) (string, error) {
	cgroupDir := "/sys/fs/cgroup/cpu/k8s.io/"
	files, err := os.ReadDir(cgroupDir)
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
	cgroupUsageData, err := os.ReadFile(cgroupUsageFile)
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
