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

package node

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/cgroup"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	defaultPodsCapacity = "500"
	Eth0SpeedFile       = "/sys/class/net/eth0/speed"
	defaultBandwidth    = "1000"
)

type CapacityManager struct {
	cpuTotal     resource.Quantity
	cpuAvailable resource.Quantity

	memTotal     resource.Quantity
	memAvailable resource.Quantity

	storageTotal     resource.Quantity
	storageAvailable resource.Quantity

	bandwidthTotal     resource.Quantity
	bandwidthAvailable resource.Quantity

	ephemeralStorageTotal     *resource.Quantity
	ephemeralStorageAvailable *resource.Quantity

	podTotal     resource.Quantity
	podAvailable resource.Quantity

	cgroupCPUQuota    *int64
	cgroupCPUPeriod   *uint64
	cgroupMemoryLimit *int64
}

func NewCapacityManager(runtime string, cfg *config.CapacityCfg, reservedResCfg *config.ReservedResourcesCfg, rootDir string, localCapacity bool) (*CapacityManager, error) {
	pa := &CapacityManager{}
	nlog.Infof("Capacity Manager, runtime: %v, capacityCfg:%v, reservedResCfg: %v, rootDir: %s, localCapacity:%v",
		runtime, cfg, reservedResCfg, rootDir, localCapacity)
	// runc or runp runtime
	if localCapacity {
		// Prioritizes the use of cgroups, i.e., the cpu and memory limits set at container startup,
		//if there are no cgroup limits, the memory and memory of the host where the container is located,
		//i.e., /proc/cpuinfo and /proc/meminfo.

		// get host memory from /proc/meminfo
		memStat, err := mem.VirtualMemory()
		if err != nil {
			return nil, fmt.Errorf("failed to get host memory state, detail-> %v", err)
		}
		// get memory limit from cgroup
		memoryLimit, err := cgroup.GetMemoryLimit(cgroup.DefaultMountPoint)
		if err == nil && memoryLimit > 0 && memoryLimit < int64(memStat.Total) {
			pa.memTotal = *resource.NewQuantity(memoryLimit, resource.BinarySI)
			pa.memAvailable = pa.memTotal.DeepCopy()
		}
		// if cgroup memory limit is not set, use host memory
		if pa.memTotal.IsZero() {
			pa.memTotal = *resource.NewQuantity(int64(memStat.Total), resource.BinarySI)
		}
		if pa.memAvailable.IsZero() {
			pa.memAvailable = *resource.NewQuantity(int64(memStat.Total), resource.BinarySI)
		}

		// One cpu, in Kubernetes, is equivalent to 1 vCPU/Core for cloud providers
		// and 1 hyperthread on bare-metal Intel processors.

		// get host cpu from /proc/cpuinfo
		cpus, err := cpu.Counts(true)
		if err != nil {
			return nil, fmt.Errorf("failed to get cpu info, detail-> %v", err)
		}
		hostCPUTotal := *resource.NewQuantity(int64(cpus), resource.BinarySI)
		cfg.CPU = strconv.Itoa(cpus)
		// get cpu limit from cgroup
		cpuQuota, cpuPeriod, err := cgroup.GetCPUQuotaAndPeriod(cgroup.DefaultMountPoint)
		if err == nil && cpuQuota > 0 && cpuPeriod > 0 {
			availableCPU := cpuQuota / cpuPeriod
			if availableCPU > 0 && availableCPU < hostCPUTotal.Value() {
				pa.cpuTotal = *resource.NewQuantity(availableCPU, resource.BinarySI)
				pa.cpuAvailable = pa.cpuTotal.DeepCopy()
			}
		}
		// if cgroup cpu limit is not set, use host cpu
		if pa.cpuTotal.IsZero() {
			pa.cpuTotal = hostCPUTotal
		}
		if pa.cpuAvailable.IsZero() {
			pa.cpuAvailable = hostCPUTotal.DeepCopy()
		}

		if cfg.Storage == "" {
			storageStat, err := disk.Usage(rootDir)
			if err != nil {
				return nil, fmt.Errorf("failed to stat disk usage[%s], detail-> %v", rootDir, err)
			}
			pa.storageAvailable = *resource.NewQuantity(int64(storageStat.Free), resource.BinarySI)
			pa.storageTotal = *resource.NewQuantity(int64(storageStat.Total), resource.BinarySI)
		}
	} else {
		// runk runtime. kuscia.yaml capacity is mandatory to set
		if cfg.CPU == "" || cfg.Memory == "" || cfg.Storage == "" {
			return nil, fmt.Errorf("capacity config is empty")
		}
		cpuQuantity, err := resource.ParseQuantity(cfg.CPU)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpu %q, detail-> %v", cfg.CPU, err)
		}

		pa.cpuTotal = cpuQuantity.DeepCopy()
		pa.cpuAvailable = cpuQuantity.DeepCopy()

		memory, err := resource.ParseQuantity(cfg.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory %q, detail-> %v", cfg.Memory, err)
		}
		if asInt64, ok := memory.AsInt64(); ok {
			memory = *resource.NewQuantity(asInt64, resource.BinarySI)
		}
		if pa.memTotal.IsZero() {
			pa.memTotal = memory.DeepCopy()
		}
		if pa.memAvailable.IsZero() {
			pa.memAvailable = memory.DeepCopy()
		}

		if pa.memTotal.Cmp(pa.memAvailable) < 0 {
			// total memory in config is smaller than available memory
			pa.memAvailable = pa.memTotal.DeepCopy()
		}
	}

	if pa.storageTotal.IsZero() || pa.storageAvailable.IsZero() {
		storageQuantity, err := resource.ParseQuantity(cfg.Storage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse storage %q, detail-> %v", cfg.Storage, err)
		}
		pa.storageTotal = storageQuantity.DeepCopy()
		pa.storageAvailable = storageQuantity.DeepCopy()
	}

	if pa.bandwidthTotal.IsZero() || pa.bandwidthAvailable.IsZero() {
		if cfg.Bandwidth == "" {
			bandwidthStr, err := getBandwidth()
			if err != nil {
				nlog.Warnf("Failed to automatically retrieve NIC bandwidth, using default value: %v", defaultBandwidth)
				cfg.Bandwidth = defaultBandwidth
			} else {
				cfg.Bandwidth = bandwidthStr
			}
		}

		bandwidth, err := resource.ParseQuantity(cfg.Bandwidth)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bandwidth %q, detail-> %v", cfg.Bandwidth, err)
		}

		pa.bandwidthTotal = bandwidth.DeepCopy()
		pa.bandwidthAvailable = bandwidth.DeepCopy()
	}

	if cfg.EphemeralStorage != "" {
		storageQuantity, err := resource.ParseQuantity(cfg.EphemeralStorage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ephemeral storage %q, detail-> %v", cfg.EphemeralStorage, err)
		}
		pa.ephemeralStorageTotal = &storageQuantity
		pa.ephemeralStorageAvailable = &storageQuantity
	}
	podsCap := cfg.Pods
	if podsCap == "" {
		podsCap = defaultPodsCapacity
	}
	pods, err := resource.ParseQuantity(podsCap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pods %q, detail-> %v", podsCap, err)
	}
	pa.podTotal = pods.DeepCopy()
	pa.podAvailable = pods.DeepCopy()

	err = pa.buildCgroupResource(runtime, reservedResCfg)
	if err != nil {
		return nil, err
	}

	return pa, nil
}

func (pa *CapacityManager) buildCgroupResource(runtime string, reservedResCfg *config.ReservedResourcesCfg) error {
	if reservedResCfg == nil {
		return nil
	}

	if runtime != config.ProcessRuntime && runtime != config.ContainerRuntime {
		return nil
	}

	reservedCPU, err := resource.ParseQuantity(reservedResCfg.CPU)
	if err != nil {
		return fmt.Errorf("failed to parse reserved cpu %q, detail-> %v", reservedResCfg.CPU, err)
	}

	if reservedCPU.MilliValue() <= 0 {
		reservedCPU, _ = resource.ParseQuantity(config.DefaultReservedCPU)
	}

	if pa.cpuAvailable.Cmp(reservedCPU) < 0 {
		return fmt.Errorf("available cpu %v is less than reserved cpu %v", pa.cpuAvailable.String(), reservedCPU.String())
	}

	cpuPeriod := uint64(100000)
	availableCPU := pa.cpuAvailable.MilliValue() - reservedCPU.MilliValue()
	cpuQuota := availableCPU * 100
	pa.cgroupCPUQuota = &cpuQuota
	pa.cgroupCPUPeriod = &cpuPeriod
	if reservedCPU.MilliValue() > 500 {
		pa.cpuAvailable.SetMilli(availableCPU)
	}

	nlog.Infof("Total cpu: %v, available cpu: %v, cpu quota: %v, cpu period: %v", pa.cpuTotal.String(), pa.cpuAvailable.String(), cpuQuota, *pa.cgroupCPUPeriod)

	reservedMemory, err := resource.ParseQuantity(reservedResCfg.Memory)
	if err != nil {
		return fmt.Errorf("failed to parse reserved memory %q, detail-> %v", reservedResCfg.Memory, err)
	}

	if reservedMemory.MilliValue() <= 0 {
		reservedMemory, _ = resource.ParseQuantity(config.DefaultReservedMemory)
	}

	if pa.memAvailable.Cmp(reservedMemory) < 0 {
		return fmt.Errorf("available memory %d is less than reserved memory %d", pa.cpuTotal.Value(), reservedCPU.Value())
	}

	availableMemory := pa.memAvailable.Value() - reservedMemory.Value()
	pa.cgroupMemoryLimit = &availableMemory
	pa.memAvailable.Set(availableMemory)

	nlog.Infof("Total memory: %v, available memory: %v", pa.memTotal.Value(), pa.memAvailable.Value())

	reservedBandwidth, err := resource.ParseQuantity(reservedResCfg.Bandwidth)
	if err != nil {
		return fmt.Errorf("failed to parse reserved bandwidth %q, detail-> %v", reservedResCfg.Bandwidth, err)
	}

	if reservedBandwidth.Value() <= 0 {
		reservedBandwidth, _ = resource.ParseQuantity(config.DefaultReservedBandwidth)
	}

	if pa.bandwidthAvailable.Cmp(reservedBandwidth) < 0 {
		return fmt.Errorf("available bandwidth %v is less than reserved bandwidth %v", pa.bandwidthAvailable.String(), reservedBandwidth.String())
	}

	availableBandwidth := pa.bandwidthAvailable.Value() - reservedBandwidth.Value()
	pa.bandwidthAvailable.Set(availableBandwidth)

	nlog.Infof("Total bandwidth: %v, available bandwidth: %v", pa.bandwidthTotal.String(), pa.bandwidthAvailable.String())

	return nil
}

// Capacity returns a resource list containing the capacity limits.
func (pa *CapacityManager) Capacity() v1.ResourceList {
	rl := v1.ResourceList{
		v1.ResourceCPU:        pa.cpuTotal,
		v1.ResourceMemory:     pa.memTotal,
		v1.ResourceStorage:    pa.storageTotal,
		"kuscia.io/bandwidth": pa.bandwidthTotal,
		"pods":                pa.podTotal,
	}
	if pa.ephemeralStorageTotal != nil {
		rl[v1.ResourceEphemeralStorage] = *pa.ephemeralStorageTotal
	}
	return rl
}

func (pa *CapacityManager) Allocatable() v1.ResourceList {
	rl := v1.ResourceList{
		v1.ResourceCPU:        pa.cpuAvailable,
		v1.ResourceMemory:     pa.memAvailable,
		v1.ResourceStorage:    pa.storageAvailable,
		"kuscia.io/bandwidth": pa.bandwidthAvailable,
		"pods":                pa.podAvailable,
	}
	if pa.ephemeralStorageAvailable != nil {
		rl[v1.ResourceEphemeralStorage] = *pa.ephemeralStorageAvailable
	}
	return rl
}

func (pa *CapacityManager) GetCgroupCPUQuota() *int64 {
	return pa.cgroupCPUQuota
}

func (pa *CapacityManager) GetCgroupCPUPeriod() *uint64 {
	return pa.cgroupCPUPeriod
}

func (pa *CapacityManager) GetCgroupMemoryLimit() *int64 {
	return pa.cgroupMemoryLimit
}

func (pa *CapacityManager) GetCPUAvailable() resource.Quantity {
	return pa.cpuAvailable
}

func (pa *CapacityManager) GetMemoryAvailable() resource.Quantity {
	return pa.memAvailable
}

func getBandwidth() (string, error) {
	// /sys/class/net/eth0/speed. This file is provided by the Linux kernel,
	// its content represents the current network interface speed, with the unit being Mbps.
	// If speed = 10, it means the bandwidth is 10 Mbps.
	data, err := os.ReadFile(Eth0SpeedFile)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", Eth0SpeedFile, err)
	}

	speedStr := strings.TrimSpace(string(data))
	speed, err := strconv.Atoi(speedStr)
	if err != nil || speed <= 0 {
		return "", fmt.Errorf("invalid speed value: %q", speedStr)
	}

	return fmt.Sprintf("%d", speed), nil
}
