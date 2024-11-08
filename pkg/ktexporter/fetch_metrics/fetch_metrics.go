package fetch_metrics

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// ContainerConfig represents the config.json structure.
type ContainerConfig struct {
	Hostname string `json:"hostname"`
}

func GetKusciaTaskPID() (map[string]string, error) {
	const containerdDir = "/home/kuscia/containerd/run/io.containerd.runtime.v2.task/k8s.io/"
	taskIDToPID := make(map[string]string)
	containerFiles, err := os.ReadDir(containerdDir)
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
			nlog.Warnf("unexpected output format for line: %s", line)
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
