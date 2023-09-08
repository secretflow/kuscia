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
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type PodsAuditor struct {
	cpuTotal     resource.Quantity
	cpuAvailable resource.Quantity

	memTotal     resource.Quantity
	memAvailable resource.Quantity

	storageTotal     resource.Quantity
	storageAvailable resource.Quantity

	podTotal     resource.Quantity
	podAvailable resource.Quantity
}

func NewPodsAuditor(config *config.AgentConfig) *PodsAuditor {
	pa := &PodsAuditor{
		cpuTotal:     resource.MustParse(config.Capacity.CPU),
		cpuAvailable: resource.MustParse(config.Capacity.CPU),

		memTotal: resource.MustParse(config.Capacity.Memory),

		podTotal:     resource.MustParse(config.Capacity.Pods),
		podAvailable: resource.MustParse(config.Capacity.Pods),
	}

	// calc memory
	if asInt64, ok := pa.memTotal.AsInt64(); ok {
		pa.memTotal = *resource.NewQuantity(asInt64, resource.BinarySI)
	}
	if memstat, err := mem.VirtualMemory(); err == nil {
		pa.memAvailable = *resource.NewQuantity(int64(memstat.Available), resource.BinarySI)
	} else {
		pa.memAvailable = pa.memTotal.DeepCopy()
	}

	if pa.memTotal.Cmp(pa.memAvailable) < 0 {
		// total memory in config is smaller than available memory
		pa.memAvailable = pa.memTotal.DeepCopy()
	}

	// calc storage
	if storageStat, err := disk.Usage(config.RootDir); err == nil {
		pa.storageAvailable = *resource.NewQuantity(int64(storageStat.Free), resource.BinarySI)
		pa.storageTotal = *resource.NewQuantity(int64(storageStat.Total), resource.BinarySI)
	} else {
		nlog.Errorf("Stat disk usage error: %v", err)
	}

	return pa
}

// Capacity returns a resource list containing the capacity limits.
func (pa *PodsAuditor) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":     pa.cpuTotal,
		"memory":  pa.memTotal,
		"storage": pa.storageTotal,
		"pods":    pa.podTotal,
	}
}

func (pa *PodsAuditor) Allocatable() v1.ResourceList {
	return v1.ResourceList{
		"cpu":     pa.cpuAvailable,
		"memory":  pa.memAvailable,
		"storage": pa.storageAvailable,
		"pods":    pa.podAvailable,
	}
}
