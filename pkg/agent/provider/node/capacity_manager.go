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
	"strconv"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

const (
	defaultPodsCapacity = "500"
)

type CapacityManager struct {
	cpuTotal     resource.Quantity
	cpuAvailable resource.Quantity

	memTotal     resource.Quantity
	memAvailable resource.Quantity

	storageTotal     resource.Quantity
	storageAvailable resource.Quantity

	podTotal     resource.Quantity
	podAvailable resource.Quantity
}

func NewCapacityManager(cfg *config.CapacityCfg, rootDir string, localCapacity bool) (*CapacityManager, error) {
	pa := &CapacityManager{}

	if localCapacity {
		memStat, err := mem.VirtualMemory()
		if err != nil {
			return nil, fmt.Errorf("failed to get host memory state, detail-> %v", err)
		}
		if cfg.Memory == "" {
			pa.memTotal = *resource.NewQuantity(int64(memStat.Total), resource.BinarySI)
		}
		pa.memAvailable = *resource.NewQuantity(int64(memStat.Available), resource.BinarySI)

		if cfg.CPU == "" {
			// One cpu, in Kubernetes, is equivalent to 1 vCPU/Core for cloud providers
			// and 1 hyperthread on bare-metal Intel processors.
			cpus, err := cpu.Counts(true)
			if err != nil {
				return nil, fmt.Errorf("failed to get cpu info, detail-> %v", err)
			}
			cfg.CPU = strconv.Itoa(cpus)
			pa.cpuTotal = *resource.NewQuantity(int64(cpus), resource.BinarySI)
			pa.cpuAvailable = pa.cpuTotal.DeepCopy()
		}

		if cfg.Storage == "" {
			storageStat, err := disk.Usage(rootDir)
			if err != nil {
				return nil, fmt.Errorf("failed to stat disk usage, detail-> %v", err)
			}
			pa.storageAvailable = *resource.NewQuantity(int64(storageStat.Free), resource.BinarySI)
			pa.storageTotal = *resource.NewQuantity(int64(storageStat.Total), resource.BinarySI)
		}
	}

	if pa.cpuTotal.IsZero() || pa.cpuAvailable.IsZero() {
		cpuQuantity, err := resource.ParseQuantity(cfg.CPU)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpu %q, detail-> %v", cfg.CPU, err)
		}

		pa.cpuTotal = cpuQuantity.DeepCopy()
		pa.cpuAvailable = cpuQuantity.DeepCopy()
	}

	if pa.memTotal.IsZero() || pa.memAvailable.IsZero() {
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
	}
	if pa.memTotal.Cmp(pa.memAvailable) < 0 {
		// total memory in config is smaller than available memory
		pa.memAvailable = pa.memTotal.DeepCopy()
	}

	if pa.storageTotal.IsZero() || pa.storageAvailable.IsZero() {
		storageQuantity, err := resource.ParseQuantity(cfg.Storage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse storage %q, detail-> %v", cfg.Storage, err)
		}

		pa.storageTotal = storageQuantity.DeepCopy()
		pa.storageAvailable = storageQuantity.DeepCopy()
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

	return pa, nil
}

// Capacity returns a resource list containing the capacity limits.
func (pa *CapacityManager) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":     pa.cpuTotal,
		"memory":  pa.memTotal,
		"storage": pa.storageTotal,
		"pods":    pa.podTotal,
	}
}

func (pa *CapacityManager) Allocatable() v1.ResourceList {
	return v1.ResourceList{
		"cpu":     pa.cpuAvailable,
		"memory":  pa.memAvailable,
		"storage": pa.storageAvailable,
		"pods":    pa.podAvailable,
	}
}
