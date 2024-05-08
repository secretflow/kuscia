//go:build linux
// +build linux

// Copyright 2024 Ant Group Co., Ltd.
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

package cgroup

import (
	"testing"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func newMockManager() (Manager, error) {
	conf := &Config{
		Group:       "kuscia.test",
		Pid:         0,
		CPUQuota:    nil,
		CPUPeriod:   nil,
		MemoryLimit: nil,
	}
	return NewManager(conf)
}

func TestNewManager(t *testing.T) {
	_, err := NewManager(nil)
	assert.NotNil(t, err)
}

func TestAddCgroup(t *testing.T) {
	m, _ := newMockManager()
	m.AddCgroup()
}

func TestUpdateCgroup(t *testing.T) {
	m, _ := newMockManager()
	m.UpdateCgroup()
}

func TestDeleteCgroup(t *testing.T) {
	m, _ := newMockManager()
	m.DeleteCgroup()
}

func TestHasPermission(t *testing.T) {
	HasPermission()
}

func TestIsCgroupExist(t *testing.T) {
	IsCgroupExist("kuscia.test", false)
}

func TestBuildCgroup2Resources(t *testing.T) {
	cpuQuota := int64(100000)
	cpuPeriod := uint64(100000)
	memoryLimit := int64(100000)
	tests := []struct {
		name        string
		cpuQuota    *int64
		cpuPeriod   *uint64
		memoryLimit *int64
		want        *cgroup2.Resources
	}{
		{
			name:        "no limits",
			cpuQuota:    nil,
			cpuPeriod:   nil,
			memoryLimit: nil,
			want:        &cgroup2.Resources{},
		},
		{
			name:        "cpu limit",
			cpuQuota:    &cpuQuota,
			cpuPeriod:   &cpuPeriod,
			memoryLimit: nil,
			want: &cgroup2.Resources{
				CPU: &cgroup2.CPU{
					Max: cgroup2.NewCPUMax(&cpuQuota, &cpuPeriod),
				},
			},
		},
		{
			name:        "memory limit",
			cpuQuota:    nil,
			cpuPeriod:   nil,
			memoryLimit: &memoryLimit,
			want: &cgroup2.Resources{
				Memory: &cgroup2.Memory{
					Max: &memoryLimit,
				},
			},
		},
		{
			name:        "both limits",
			cpuQuota:    &cpuQuota,
			cpuPeriod:   &cpuPeriod,
			memoryLimit: &memoryLimit,
			want: &cgroup2.Resources{
				CPU: &cgroup2.CPU{
					Max: cgroup2.NewCPUMax(&cpuQuota, &cpuPeriod),
				},
				Memory: &cgroup2.Memory{
					Max: &memoryLimit,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCgroup2Resources(tt.cpuQuota, tt.cpuPeriod, tt.memoryLimit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildCgroup1Resources(t *testing.T) {
	cpuQuota := int64(100000)
	cpuPeriod := uint64(100000)
	memoryLimit := int64(100000)

	tests := []struct {
		name          string
		cpuQuota      *int64
		cpuPeriod     *uint64
		memoryLimit   *int64
		expectedValue *specs.LinuxResources
	}{
		{
			name:          "no limits",
			cpuQuota:      nil,
			cpuPeriod:     nil,
			memoryLimit:   nil,
			expectedValue: &specs.LinuxResources{},
		},
		{
			name:        "cpu limit",
			cpuQuota:    &cpuQuota,
			cpuPeriod:   &cpuPeriod,
			memoryLimit: nil,
			expectedValue: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Quota:  &cpuQuota,
					Period: &cpuPeriod,
				},
			},
		},
		{
			name:        "memory limit",
			cpuQuota:    nil,
			cpuPeriod:   nil,
			memoryLimit: &memoryLimit,
			expectedValue: &specs.LinuxResources{
				Memory: &specs.LinuxMemory{Limit: &memoryLimit},
			},
		},
		{
			name:        "both limits",
			cpuQuota:    &cpuQuota,
			cpuPeriod:   &cpuPeriod,
			memoryLimit: &memoryLimit,
			expectedValue: &specs.LinuxResources{
				CPU: &specs.LinuxCPU{
					Quota:  &cpuQuota,
					Period: &cpuPeriod,
				},
				Memory: &specs.LinuxMemory{Limit: &memoryLimit}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCgroup1Resources(tt.cpuQuota, tt.cpuPeriod, tt.memoryLimit)
			assert.Equal(t, tt.expectedValue, got)
		})
	}
}

func TestGetMemoryLimit(t *testing.T) {
	limit, err := GetMemoryLimit("kuscia.test/test")
	assert.Equal(t, int64(0), limit)
	assert.NotNil(t, err)
}

func TestGetCPUQuotaAndPeriod(t *testing.T) {
	quota, period, err := GetCPUQuotaAndPeriod("kuscia.test/test")
	assert.Equal(t, int64(0), quota)
	assert.Equal(t, int64(0), period)
	assert.NotNil(t, err)
}

func TestParseCgroup2MemoryLimit(t *testing.T) {
	parseCgroup2MemoryLimit(DefaultMountPoint)
}

func TestParseCgroup1MemoryLimit(t *testing.T) {
	parseCgroup1MemoryLimit(DefaultMountPoint)
}

func TestParseCgroup2CPUQuotaAndPeriod(t *testing.T) {
	parseCgroup2CPUQuotaAndPeriod(DefaultMountPoint)
}

func TestParseCgroup1CPUQuotaAndPeriod(t *testing.T) {
	parseCgroup1CPUQuotaAndPeriod(DefaultMountPoint)
}
