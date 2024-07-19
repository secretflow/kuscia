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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	v1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/agent/utils/nodeutils"
	"github.com/secretflow/kuscia/pkg/utils/math"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/runtime"
)

const (
	MemoryPressureThreshold        = 0.9                    // 90%
	DiskPressureThreshold          = 95                     // 95%
	DiskPressureMinFreeSize uint64 = 3 * 1024 * 1024 * 1024 // 3G

	DiskOutMinFreeSize  = 100 * 1024 * 1024 // 100MB
	DiskOutMinFreeInode = 1000              // 1000 inodes

	minRefreshKernelParamInterval = time.Minute * 5
)

type GenericNodeDependence struct {
	BaseNodeDependence
	DiskPressurePath string
}

type GenericNodeProvider struct {
	diskPressurePath string

	cachedKernelParamOK             bool
	cachedKernelParamMsg            string
	cachedKernelParamLastUpdateTime time.Time

	*BaseNode
}

func NewGenericNodeProvider(dep *GenericNodeDependence) *GenericNodeProvider {
	gnp := &GenericNodeProvider{
		diskPressurePath:                dep.DiskPressurePath,
		cachedKernelParamLastUpdateTime: time.Now().Add(-1 * time.Hour),
	}
	gnp.BaseNode = newBaseNode(&dep.BaseNodeDependence)

	return gnp
}

func (gnp *GenericNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (gnp *GenericNodeProvider) SetStatusUpdateCallback(ctx context.Context, f func(*v1.Node)) {
}

func (gnp *GenericNodeProvider) ConfigureNode(ctx context.Context, name string) *v1.Node {
	node := gnp.configureCommonNode(ctx, name)
	gnp.RefreshNodeStatus(ctx, &node.Status)

	nlog.Infof("Configure generic node %q successfully", name)

	return node
}

// refreshDiskCondition checks whether the disk capacity is under pressure.
// return:
//  1. does disk has pressure? [bool]
//  2. does out of disk? [bool]
//  3. message of disk pressure status [string]
//  4. message of disk out status [string]
func (gnp *GenericNodeProvider) refreshDiskCondition(name, path string) (bool, bool, string, string) {
	du, err := disk.Usage(path)
	if err != nil {
		nlog.Warnf("Get disk usage info fail, volume=%v, path=%v, err=%v", name, path, err)
		msg := fmt.Sprintf("%v: read disk info fail", name)
		return false, false, msg, msg
	}

	// disk free size <= 5GB and free < 5%, inode <= 5%
	diskPressure := (du.InodesUsedPercent >= DiskPressureThreshold) ||
		(du.UsedPercent >= DiskPressureThreshold && du.Free <= DiskPressureMinFreeSize)

	outOfDisk := du.Free <= DiskOutMinFreeSize || du.InodesFree <= DiskOutMinFreeInode
	// FSType has bug, so we not use it in message
	// du.Used+du.Free=du.Total-reservedBlocks
	pressureMsg := fmt.Sprintf("@%v(%s): space=%v/%v(%.1f%%) inode=%v/%v(%.1f%%)", name, path,
		math.ByteCountBinary(int64(du.Used)),
		math.ByteCountBinary(int64(du.Used+du.Free)), du.UsedPercent,
		math.ByteCountDecimalRaw(int64(du.InodesUsed)),
		math.ByteCountDecimalRaw(int64(du.InodesTotal)), du.InodesUsedPercent)
	outMsg := fmt.Sprintf("@%v: free_space=%v, free_inode=%v",
		name, math.ByteCountBinary(int64(du.Free)), math.ByteCountDecimalRaw(int64(du.InodesFree)))

	return diskPressure, outOfDisk, pressureMsg, outMsg
}

func (gnp *GenericNodeProvider) refreshKernelParamsCondition() (bool, string) {
	if time.Since(gnp.cachedKernelParamLastUpdateTime) > minRefreshKernelParamInterval {
		isOK := true
		msg := []string{}
		for _, param := range runtime.CurrentKernel().List() {
			isOK = isOK && param.IsMatch
			if param.IsMatch {
				msg = append(msg, fmt.Sprintf("%s=%s[OK]", param.Key, param.Value))
			} else {
				msg = append(msg, fmt.Sprintf("%s=%s[ERR]", param.Key, param.Value))
			}
		}

		gnp.cachedKernelParamOK = isOK
		gnp.cachedKernelParamMsg = strings.Join(msg, ";")
		gnp.cachedKernelParamLastUpdateTime = time.Now()
	}

	return gnp.cachedKernelParamOK, gnp.cachedKernelParamMsg
}

// refreshNodeConditions refreshes node condition.
// return: whether the condition changes
func (gnp *GenericNodeProvider) refreshNodeConditions(ctx context.Context, st *v1.NodeStatus) bool {
	// memory condition
	var memChanged bool
	if memory, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		nlog.Warnf("Failed to refresh memory condition: %v", err)
	} else {
		var memoryPressure float64
		if memory.Total > 0 {
			memoryPressure =
				(float64(memory.Total) - float64(memory.Available)) / float64(memory.Total)
		}
		if memoryPressure >= MemoryPressureThreshold {
			st.Conditions, memChanged = nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
				Reason: "AgentHasMemoryPressure",
				Message: fmt.Sprintf("Memory is about to run out, total=%v, available=%v",
					math.ByteCountBinary(int64(memory.Total)),
					math.ByteCountBinary(int64(memory.Available))),
			})
		} else {
			st.Conditions, memChanged = nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionFalse,
				Reason: "AgentHasSufficientMemory",
				Message: fmt.Sprintf("Agent has sufficient memory available, total=%v, available=%v",
					math.ByteCountBinary(int64(memory.Total)),
					math.ByteCountBinary(int64(memory.Available))),
			})
		}
	}

	// disk
	var diskPressureChanged, diskOutChanged bool
	diskPressure, outOfDisk, pressureMsg, outMsg := gnp.refreshDiskCondition("agent_volume", gnp.diskPressurePath)
	if diskPressure {
		st.Conditions, diskPressureChanged = nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeDiskPressure,
			Status:  v1.ConditionTrue,
			Reason:  "AgentHasDiskPressure",
			Message: fmt.Sprintf("Disk is about to run out. %v", pressureMsg),
		})
	} else {
		st.Conditions, diskPressureChanged = nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeDiskPressure,
			Status:  v1.ConditionFalse,
			Reason:  "AgentHasNoDiskPressure",
			Message: fmt.Sprintf("Agent has no disk pressure. %v", pressureMsg),
		})
	}

	if outOfDisk {
		st.Conditions, diskOutChanged =
			nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "OutOfDisk",
				Status:  v1.ConditionTrue,
				Reason:  "AgentIsOutOfDisk",
				Message: fmt.Sprintf("Disk is almost used up. %v", outMsg),
			})
	} else {
		st.Conditions, diskOutChanged =
			nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "OutOfDisk",
				Status:  v1.ConditionFalse,
				Reason:  "AgentHasSufficientDisk",
				Message: fmt.Sprintf("Agent has sufficient disk space available. %v", outMsg),
			})
	}

	var kernelParamsChanged bool
	kernalParamOK, kerunalParamMsg := gnp.refreshKernelParamsCondition()
	if kernalParamOK {
		st.Conditions, kernelParamsChanged =
			nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "Kernel-Params",
				Status:  v1.ConditionTrue,
				Reason:  "Kernel parameters satisfy kuscia recommended requirements",
				Message: kerunalParamMsg,
			})
	} else {
		st.Conditions, kernelParamsChanged =
			nodeutils.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "Kernel-Params",
				Status:  v1.ConditionFalse,
				Reason:  "Kernel parameters not satisfy kuscia recommended requirements",
				Message: kerunalParamMsg,
			})
	}

	return memChanged || diskPressureChanged || diskOutChanged || kernelParamsChanged
}

func (gnp *GenericNodeProvider) RefreshNodeStatus(ctx context.Context, nodeStatus *v1.NodeStatus) bool {
	condChange := gnp.refreshNodeConditions(ctx, nodeStatus)

	nlog.Debugf("Refresh node status finish, condition_changed=%v", condChange)
	return condChange
}
