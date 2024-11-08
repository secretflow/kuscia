package container_stats

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

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
			nlog.Warnf("unexpected output format for line: %s", line)
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
