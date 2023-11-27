// measure the usage of system resources used by monitor.go
package sysmon

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GetMemoryUsage get the memory usage of the monitor program
func GetMemoryUsage(pid string) (float64, float64, float64) {
	// get the memory usage from the pid
	pidstatCmd := exec.Command("pidstat", "-p", pid, "-r")
	var output bytes.Buffer
	pidstatCmd.Stdout = &output
	if err := pidstatCmd.Run(); err != nil {
		log.Fatal(err)
	}

	// parse statistics from pidstat
	lines := strings.Split(output.String(), "\n")
	var vsz float64
	var rss float64
	var memPercent float64

	for _, line := range lines {
		if strings.Contains(line, "%MEM") {
			continue
		} else {
			fields := strings.Fields(line)
			if len(fields) >= 9 {
				vsz, _ = strconv.ParseFloat(fields[5], 64)
				rss, _ = strconv.ParseFloat(fields[6], 64)
				memPercent, _ = strconv.ParseFloat(fields[7], 64)
			}
		}
	}
	return vsz, rss, memPercent
}

// GetNetworkIOUsage get the network IO usage of the monitor program
func GetNetworkIOUsage(pid string) (int, int) {
	// get the network data from ioutil
	netFilePath := fmt.Sprintf("/proc/%s/net/dev", pid)
	netData, err := ioutil.ReadFile(netFilePath)
	if err != nil {
		log.Fatal(err)
	}
	// parse the network data
	var receivedBytes int
	var sentBytes int
	lines := strings.Split(string(netData), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") && strings.Contains(line, "eth0") {
			fields := strings.Fields(line)
			if len(fields) >= 17 {
				receivedBytes, _ = strconv.Atoi(fields[1])
				sentBytes, _ = strconv.Atoi(fields[9])
				break
			}
		}
	}
	return sentBytes, receivedBytes
}

// GetCPUUsage get the CPU usage of the monitor program
func GetCPUUsage(pid string) (float64, float64, float64) {
	// get CPU usage from pidstat
	pidstatCmd := exec.Command("pidstat", "-p", pid)
	var output bytes.Buffer
	pidstatCmd.Stdout = &output
	if err := pidstatCmd.Run(); err != nil {
		log.Fatal(err)
	}

	// parse the data from pidstat
	lines := strings.Split(output.String(), "\n")
	var usrUsagePercent float64
	var sysUsagePercent float64
	var cpuUsagePercent float64
	for _, line := range lines {
		if strings.Contains(line, pid) {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				usrUsagePercent, _ = strconv.ParseFloat(fields[3], 64)
				sysUsagePercent, _ = strconv.ParseFloat(fields[4], 64)
				cpuUsagePercent, _ = strconv.ParseFloat(fields[7], 64)
			}
		}
	}
	return usrUsagePercent, sysUsagePercent, cpuUsagePercent
}

// GetSystemResourceUsage get the usage of system resources from the monitor program
func GetSystemResourceUsage(pid string, initSysRes []float64, sysRes []float64) ([]float64, []float64) {
	// get memory usage
	vsz, rss, memPercent := GetMemoryUsage(pid)
	sysRes = append(sysRes, vsz)
	sysRes = append(sysRes, rss)
	sysRes = append(sysRes, memPercent)
	// get network IO usage
	sentBytes, receivedBytes := GetNetworkIOUsage(pid)
	sysRes = append(sysRes, float64(sentBytes))
	sysRes = append(sysRes, float64(receivedBytes))
	// get cpu usage
	usrUsagePercent, sysUsagePercent, cpuUsagePercent := GetCPUUsage(pid)
	sysRes = append(sysRes, usrUsagePercent)
	sysRes = append(sysRes, sysUsagePercent)
	sysRes = append(sysRes, cpuUsagePercent)
	if len(initSysRes) == 0 {
		initSysRes = make([]float64, len(sysRes))
		copy(initSysRes, sysRes)
	}
	//sysRes[0] = sysRes[0] - initSysRes[0]
	sysRes[0] = sysRes[0] - initSysRes[0]
	sysRes[1] = sysRes[1] - initSysRes[1]
	sysRes[3] = sysRes[3] - initSysRes[3]
	sysRes[4] = sysRes[4] - initSysRes[4]
	return initSysRes, sysRes
}

// LogSystemResourceUsage write the system usage into a file
func LogSystemResourceUsage(sysOutput *os.File, sysRes []float64) {
	for _, metric := range sysRes {
		_, err := sysOutput.WriteString(fmt.Sprintf("%f", metric) + ";")
		if err != nil {
			log.Fatalln("record the usage of system resources", err)
			return
		}
	}
	sysOutput.WriteString("\n")
}
