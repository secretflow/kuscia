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

const (
	DefaultMountPoint = "/sys/fs/cgroup"
	KusciaAppsGroup   = "/kuscia.apps"
	K8sIOGroup        = "/k8s.io"
)

const (
	// MaxMemoryLimit represents the unlimited cgroup memory.limit_in_bytes value
	MaxMemoryLimit int64 = 9223372036854771712
	// MaxCPUQuota represents the unlimited cgroup cpu.cfs_quota_us value
	MaxCPUQuota int64 = -1
)

type Config struct {
	Group       string
	Pid         uint64
	CPUQuota    *int64
	CPUPeriod   *uint64
	MemoryLimit *int64
}

type Manager interface {
	AddCgroup() error
	UpdateCgroup() error
	DeleteCgroup() error
}
