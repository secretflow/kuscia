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

//nolint:all
package runtime

import (
	"os"
	"strconv"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// ref: https://fossd.anu.edu.au/linux/latest/source/include/uapi/linux/capability.h
// Linux Kernel >= 3.5 (CentOS >= 7, Ubuntu >= 12.10)
// const CAP_PRIVILEGED uint64 = 0x1fffffffff
// Linux Kernel >= 2.6.25 (CentOS >= 6, Ubuntu >= 8.10)
const CAP_PRIVILEGED uint64 = 0x3ffffffff
const CAP_SYS_ADMIN uint64 = 1 << 21
const CAP_SYS_RESOURCE uint64 = 1 << 24

type RuntimePermission interface {
	// is running in container
	IsInContainer() bool

	// has privileged
	HasPrivileged() bool

	// has cap_sys_admin
	HasSysAdminPermission() bool

	// has sys mount permission
	HasSysMountPermission() bool

	HasSetOOMScorePermission() bool

	HasChrootPermission() bool
}

type kusciaRuntimePermission struct {
	inContainer *bool
	myCapEff    *uint64
}

var Permission RuntimePermission

func init() {
	Permission = &kusciaRuntimePermission{}
}

func (k *kusciaRuntimePermission) IsInContainer() bool {
	if k.inContainer == nil {
		inContainer := false
		k.inContainer = &inContainer
		data, err := os.ReadFile("/proc/self/cgroup")
		if err == nil {
			cgroupStr := string(data)
			// Check if the cgroup file contains any container identifiers
			containerIdentifiers := []string{"docker", "lxc", "kubepods"}
			for _, id := range containerIdentifiers {
				if strings.Contains(cgroupStr, id) {
					inContainer = true
					break
				}
			}
		}

		nlog.Infof("[Runtime] Running In Container = %v", inContainer)
	}
	return *k.inContainer
}

func (k *kusciaRuntimePermission) HasPrivileged() bool {
	return k.hasCap(CAP_PRIVILEGED)
}

func (k *kusciaRuntimePermission) HasSysAdminPermission() bool {
	return k.hasCap(CAP_SYS_ADMIN)
}

func (k *kusciaRuntimePermission) HasSysMountPermission() bool {
	panic("not implement")
}

func (k *kusciaRuntimePermission) HasSetOOMScorePermission() bool {
	return k.hasCap(CAP_SYS_RESOURCE)
}

func (k *kusciaRuntimePermission) HasChrootPermission() bool {
	panic("not implement")
}

func (k *kusciaRuntimePermission) getMySelfCap() uint64 {
	if k.myCapEff == nil {
		capEff := uint64(0)
		k.myCapEff = &capEff
		data, err := os.ReadFile("/proc/self/status")
		if err == nil {
			statusStr := string(data)

			if pos := strings.Index(statusStr, "CapEff:\t"); pos != -1 {
				if cap, _, found := strings.Cut(statusStr[pos+len("CapEff:\t"):], "\n"); found {
					nlog.Infof("[CapEff]: /proc/self/status capeff=%s", cap)
					if v, err := strconv.ParseUint(cap, 16, 64); err == nil {
						capEff = v
					}
				}
			}

		}
	}
	return *k.myCapEff
}

func (k *kusciaRuntimePermission) hasCap(cap uint64) bool {
	return (k.getMySelfCap() & cap) == cap
}
