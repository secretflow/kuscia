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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func NewManager(conf *Config) (Manager, error) {
	if conf == nil || conf.Group == "" {
		return nil, fmt.Errorf("cgroup group can't be empty")
	}

	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified, cgroups.Hybrid:
		return newKCgroup2(conf)
	case cgroups.Legacy:
		return newKCgroup1(conf)
	default:
		return nil, fmt.Errorf("unsupported cgroup version: %v", mode)
	}
}

type KCgroup1 struct {
	*Config
}

func newKCgroup1(conf *Config) (*KCgroup1, error) {
	m := &KCgroup1{conf}
	return m, nil
}

func (m *KCgroup1) AddCgroup() error {
	resources := buildCgroup1Resources(m.CPUQuota, m.CPUPeriod, m.MemoryLimit)
	cg, err := cgroup1.New(cgroup1.StaticPath(m.Group), resources)
	if err != nil {
		return err
	}

	if m.Pid > 0 {
		return cg.AddProc(m.Pid)
	}
	return nil
}

func (m *KCgroup1) UpdateCgroup() error {
	if err := paths.EnsurePath(filepath.Join(DefaultMountPoint, "/cpu", m.Group), false); err != nil {
		return err
	}

	cg, err := cgroup1.Load(cgroup1.StaticPath(m.Group))
	if err != nil {
		return err
	}
	resources := buildCgroup1Resources(m.CPUQuota, m.CPUPeriod, m.MemoryLimit)
	return cg.Update(resources)
}

func (m *KCgroup1) DeleteCgroup() error {
	cg, err := cgroup1.Load(cgroup1.StaticPath(m.Group))
	if err != nil {
		return err
	}
	return cg.Delete()
}

type KCgroup2 struct {
	*Config
}

func newKCgroup2(conf *Config) (*KCgroup2, error) {
	m := &KCgroup2{conf}
	return m, nil
}

func (m *KCgroup2) AddCgroup() error {
	resources := buildCgroup2Resources(m.CPUQuota, m.CPUPeriod, m.MemoryLimit)
	cg, err := cgroup2.NewManager(DefaultMountPoint, m.Group, resources)
	if err != nil {
		return err
	}

	if m.Pid > 0 {
		return cg.AddProc(m.Pid)
	}
	return nil
}

func (m *KCgroup2) UpdateCgroup() error {
	if err := paths.EnsurePath(filepath.Join(DefaultMountPoint, m.Group), false); err != nil {
		return err
	}

	cg, err := cgroup2.Load(m.Group)
	if err != nil {
		return err
	}
	resources := buildCgroup2Resources(m.CPUQuota, m.CPUPeriod, m.MemoryLimit)
	return cg.Update(resources)
}

func (m *KCgroup2) DeleteCgroup() error {
	cg, err := cgroup2.Load(m.Group)
	if err != nil {
		return err
	}
	return cg.Delete()
}

func HasPermission() bool {
	return IsCgroupExist(KusciaAppsGroup, true)
}

func IsCgroupExist(group string, autoCreate bool) bool {
	groupPath := ""
	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified, cgroups.Hybrid:
		groupPath = filepath.Join(DefaultMountPoint, group)
	case cgroups.Legacy:
		groupPath = filepath.Join(DefaultMountPoint, "/cpu", group)
	default:
		nlog.Warnf("Unsupported cgroup version: %v", mode)
		return false
	}

	err := paths.EnsurePath(groupPath, autoCreate)
	if err != nil {
		nlog.Infof("Cgroup path does not exist, %v", err)
		return false
	}

	return true
}

func buildCgroup2Resources(cpuQuota *int64, cpuPeriod *uint64, memoryLimit *int64) *cgroup2.Resources {
	resources := &cgroup2.Resources{}
	if (cpuQuota != nil && *cpuQuota != 0) || (cpuPeriod != nil && *cpuPeriod != 0) {
		resources.CPU = &cgroup2.CPU{
			Max: cgroup2.NewCPUMax(cpuQuota, cpuPeriod),
		}
	}

	if memoryLimit != nil && *memoryLimit != 0 {
		resources.Memory = &cgroup2.Memory{
			Max: memoryLimit,
		}
	}
	return resources
}

func buildCgroup1Resources(cpuQuota *int64, cpuPeriod *uint64, memoryLimit *int64) *specs.LinuxResources {
	resources := &specs.LinuxResources{}
	if (cpuQuota != nil && *cpuQuota != 0) || (cpuPeriod != nil && *cpuPeriod != 0) {
		resources.CPU = &specs.LinuxCPU{
			Quota:  cpuQuota,
			Period: cpuPeriod,
		}
	}

	if memoryLimit != nil && *memoryLimit != 0 {
		resources.Memory = &specs.LinuxMemory{
			Limit: memoryLimit,
		}
	}
	return resources
}

func GetMemoryLimit(group string) (int64, error) {
	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified, cgroups.Hybrid:
		return parseCgroup2MemoryLimit(group)
	case cgroups.Legacy:
		return parseCgroup1MemoryLimit(group)
	default:
		return 0, fmt.Errorf("unsupported cgroup version: %v", mode)
	}
}

func parseCgroup2MemoryLimit(group string) (limit int64, err error) {
	content, err := os.ReadFile(filepath.Join(group, "/memory.max"))
	if err != nil {
		return 0, err
	}

	contentStr := strings.TrimSpace(string(content))
	if contentStr == "max" {
		limit = MaxMemoryLimit
	} else {
		limit, err = strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory limit content: %s", content)
		}
	}

	return limit, nil
}

func parseCgroup1MemoryLimit(group string) (int64, error) {
	content, err := os.ReadFile(filepath.Join(group, "/memory/memory.limit_in_bytes"))
	if err != nil {
		return 0, err
	}

	limit, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit content: %s", content)
	}

	return limit, nil
}

func GetMemoryUsage(group string) (int64, error) {
	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified, cgroups.Hybrid:
		return parseCgroup2MemoryUsage(group)
	case cgroups.Legacy:
		return parseCgroup1MemoryUsage(group)
	default:
		return 0, fmt.Errorf("unsupported cgroup version: %v", mode)
	}
}

// parseMemoryStat parses memory.stat file content into a map
func parseMemoryStat(content []byte) map[string]int64 {
	stats := make(map[string]int64)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 2 {
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				stats[fields[0]] = value
			}
		}
	}
	return stats
}

func parseCgroup2MemoryUsage(group string) (usage int64, err error) {
	// Get current memory usage
	currentContent, err := os.ReadFile(filepath.Join(group, "memory.current"))
	if err != nil {
		return 0, err
	}

	usage, err = strconv.ParseInt(strings.TrimSpace(string(currentContent)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory usage content: %s", currentContent)
	}

	// Read and parse memory.stat file
	statContent, err := os.ReadFile(filepath.Join(group, "memory.stat"))
	if err != nil {
		return 0, err
	}

	stats := parseMemoryStat(statContent)
	// Subtract inactive_file from usage
	if inactiveFile, ok := stats["inactive_file"]; ok && inactiveFile < usage {
		usage -= inactiveFile
	}

	return usage, nil
}

func parseCgroup1MemoryUsage(group string) (usage int64, err error) {
	// Get current memory usage
	content, err := os.ReadFile(filepath.Join(group, "memory/memory.usage_in_bytes"))
	if err != nil {
		return 0, err
	}

	usage, err = strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory usage content: %s", content)
	}

	// Read and parse memory.stat file
	statContent, err := os.ReadFile(filepath.Join(group, "memory/memory.stat"))
	if err != nil {
		return 0, err
	}

	stats := parseMemoryStat(statContent)
	// Subtract total_inactive_file from usage
	if totalInactiveFile, ok := stats["total_inactive_file"]; ok && totalInactiveFile < usage {
		usage -= totalInactiveFile
	}

	return usage, nil
}

func GetCPUQuotaAndPeriod(group string) (quota int64, period int64, err error) {
	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified, cgroups.Hybrid:
		return parseCgroup2CPUQuotaAndPeriod(group)
	case cgroups.Legacy:
		return parseCgroup1CPUQuotaAndPeriod(group)
	default:
		return 0, 0, fmt.Errorf("unsupported cgroup version: %v", mode)
	}
}

func parseCgroup2CPUQuotaAndPeriod(group string) (quota int64, period int64, err error) {
	content, err := os.ReadFile(filepath.Join(group, "/cpu.max"))
	if err != nil {
		return 0, 0, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(content)), " ", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cpu.max content: %s", content)
	}

	if parts[0] == "max" {
		quota = MaxCPUQuota
	} else {
		quota, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid cpu quota content: %s", parts[0])
		}
	}

	period, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cpu period content: %s", parts[0])
	}

	return quota, period, nil
}

func parseCgroup1CPUQuotaAndPeriod(group string) (quota int64, period int64, err error) {
	content, err := os.ReadFile(filepath.Join(group, "/cpu/cpu.cfs_quota_us"))
	if err != nil {
		return 0, 0, err
	}

	quota, err = strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cpu quota content: %s", content)
	}

	content, err = os.ReadFile(filepath.Join(group, "/cpu/cpu.cfs_period_us"))
	if err != nil {
		return 0, 0, err
	}

	period, err = strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cpu period content: %s", content)
	}

	return quota, period, nil
}
