// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestNewGenericNodeProvider(t *testing.T) {
	nonEmptyCfg := config.CapacityCfg{
		CPU:     "1",
		Memory:  "1000000000",
		Pods:    "100",
		Storage: "100G",
	}

	tests := []struct {
		localCapacity bool
		cfg           config.CapacityCfg
		hasErr        bool
		useCfg        bool
	}{
		{
			localCapacity: true,
			cfg:           nonEmptyCfg,
			useCfg:        true,
		},
		{
			localCapacity: true,
			cfg:           config.CapacityCfg{},
			useCfg:        false,
		},
		{
			localCapacity: false,
			cfg:           nonEmptyCfg,
			useCfg:        true,
		},
		{
			localCapacity: false,
			cfg:           config.CapacityCfg{},
			hasErr:        true,
		},
	}

	rootDir := t.TempDir()

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			cp, err := NewCapacityManager(config.ContainerRuntime, &tt.cfg, nil, rootDir, tt.localCapacity)
			if tt.hasErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.True(t, cp.cpuAvailable.Equal(cp.cpuTotal) || cp.cpuAvailable.Cmp(cp.cpuTotal) < 0)
			assert.True(t, cp.podAvailable.Equal(cp.podTotal))
			if tt.useCfg {
				assert.True(t, cp.storageAvailable.Equal(cp.storageTotal))
				assert.True(t, cp.memAvailable.Equal(cp.memTotal) || cp.memAvailable.Cmp(cp.memTotal) < 0)
				cfgCPU, _ := strconv.Atoi(tt.cfg.CPU)
				cpuAvailable, _ := strconv.Atoi(cp.cpuAvailable.String())
				assert.True(t, cfgCPU >= cpuAvailable)
				assert.Equal(t, tt.cfg.Storage, cp.storageAvailable.String())
				assert.Equal(t, tt.cfg.Pods, cp.podAvailable.String())
			}
		})
	}
}

func TestBuildCgroupResource(t *testing.T) {
	pointerToInt64 := func(i *int64) int64 {
		if i == nil {
			return 0
		}
		return *i
	}

	pointerToUint64 := func(i *uint64) int64 {
		if i == nil {
			return 0
		}
		return int64(*i)
	}

	tests := []struct {
		runtime               string
		reservedResCfg        *config.ReservedResourcesCfg
		wantCpuAvailable      int64
		wantMemAvailable      int64
		wantCgroupCPUQuota    int64
		wantCgroupCPUPeriod   int64
		wantCgroupMemoryLimit int64
	}{
		{config.ContainerRuntime, nil, 4, 838860800, 0, 0, 0},
		{"", &config.ReservedResourcesCfg{CPU: "500m", Memory: "500Mi"}, 4, 838860800, 0, 0, 0},
		{config.K8sRuntime, &config.ReservedResourcesCfg{CPU: "500m", Memory: "500Mi"}, 4, 838860800, 0, 0, 0},
		{config.ContainerRuntime, &config.ReservedResourcesCfg{CPU: "500m", Memory: "500Mi"}, 4, 314572800, 350000, 100000, 314572800},
		{config.ProcessRuntime, &config.ReservedResourcesCfg{CPU: "600m", Memory: "500Mi"}, 4, 314572800, 340000, 100000, 314572800},
	}

	for _, tt := range tests {
		pa := &CapacityManager{
			cpuTotal:     *resource.NewQuantity(4, resource.BinarySI),
			cpuAvailable: *resource.NewQuantity(4, resource.BinarySI),
			memTotal:     *resource.NewQuantity(1073741824, resource.BinarySI), // 1024Mi
			memAvailable: *resource.NewQuantity(838860800, resource.BinarySI),  // 800Mi
		}

		err := pa.buildCgroupResource(tt.runtime, tt.reservedResCfg)
		assert.Nil(t, err)
		assert.Equal(t, tt.wantCpuAvailable, pa.cpuAvailable.Value())
		assert.Equal(t, tt.wantMemAvailable, pa.memAvailable.Value())
		assert.Equal(t, tt.wantCgroupCPUQuota, pointerToInt64(pa.cgroupCPUQuota))
		assert.Equal(t, tt.wantCgroupCPUPeriod, pointerToUint64(pa.cgroupCPUPeriod))
		assert.Equal(t, tt.wantCgroupMemoryLimit, pointerToInt64(pa.cgroupMemoryLimit))
	}
}

func TestGetCgroupCPUQuota(t *testing.T) {
	quota := int64(100000)
	pa := &CapacityManager{
		cgroupCPUQuota: &quota,
	}

	got := pa.GetCgroupCPUQuota()
	assert.Equal(t, quota, *got)
}

func TestGetCgroupCPUPeriod(t *testing.T) {
	period := uint64(100000)
	pa := &CapacityManager{
		cgroupCPUPeriod: &period,
	}

	got := pa.GetCgroupCPUPeriod()
	assert.Equal(t, period, *got)
}

func TestGetCgroupMemoryLimit(t *testing.T) {
	limit := int64(100000)
	pa := &CapacityManager{
		cgroupMemoryLimit: &limit,
	}

	got := pa.GetCgroupMemoryLimit()
	assert.Equal(t, limit, *got)
}
