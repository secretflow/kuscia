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
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/framework"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/math"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	MemoryPressureThreshold = 0.9 // 90%
	DiskPressureThreshold   = 95  // 95%

	DiskOutMinFreeSize  = 100 * 1024 * 1024 // 100MB
	DiskOutMinFreeInode = 1000              // 1000 inodes

	nodeRoleLabelKey       = "kubernetes.io/role"
	nodeAPIVersionLabelKey = "kubernetes.io/apiVersion"
	domainLabelKey         = "domain"
)

type GenericNodeConfig struct {
	Namespace    string
	NodeName     string
	NodeIP       string
	APIVersion   string
	AgentVersion string
	AgentConfig  *config.AgentConfig
	PodsAuditor  *PodsAuditor
}

type GenericNodeProvider struct {
	name string
	ip   string
	ns   string
	port string

	APIVersion   string
	AgentVersion string

	podsAuditor *PodsAuditor

	config   *config.AgentConfig
	notifier func(*v1.Node)

	isAgentReady      bool
	agentReadyMessage string
}

func NewNodeProvider(cfg *GenericNodeConfig) (*GenericNodeProvider, error) {
	np := &GenericNodeProvider{
		name: cfg.NodeName,
		ip:   cfg.NodeIP,
		ns:   cfg.Namespace,

		APIVersion:   cfg.APIVersion,
		AgentVersion: cfg.AgentVersion,

		podsAuditor: cfg.PodsAuditor,
		config:      cfg.AgentConfig,

		isAgentReady:      false,
		agentReadyMessage: "Agent is starting",
	}

	return np, nil
}

func (np *GenericNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (np *GenericNodeProvider) SetStatusUpdateCallback(ctx context.Context, f func(*v1.Node)) {
	np.notifier = f
}

func (np *GenericNodeProvider) ConfigureNode(ctx context.Context) *v1.Node {
	nlog.Debug("Start generate k8s node")

	nodeInfo := v1.NodeSystemInfo{
		Architecture:    runtime.GOARCH,
		KubeletVersion:  np.AgentVersion,
		OperatingSystem: runtime.GOOS,
	}

	hi, err := host.InfoWithContext(ctx)
	if err == nil {
		if hi.KernelArch != "" {
			nodeInfo.Architecture = hi.KernelArch
		}
		nodeInfo.BootID = fmt.Sprintf("%v-%v", hi.BootTime, time.Now().UnixNano())
		nodeInfo.MachineID = hi.HostID
		nodeInfo.KernelVersion = hi.KernelVersion
		prefix := ""
		if hi.VirtualizationSystem != "" {
			prefix = hi.VirtualizationSystem + "://"
		}
		nodeInfo.OSImage = fmt.Sprintf("%s%s:%s", prefix, joinStrings([]string{hi.OS, hi.Platform}, "/"), hi.PlatformVersion)
		suffix := joinStrings([]string{hi.PlatformFamily, hi.VirtualizationRole}, ", ")
		if suffix != "" {
			nodeInfo.OSImage += fmt.Sprintf(" (%s)", suffix)
		}
	} else {
		nlog.Errorf("Read system information fail: %v", err)
		nodeInfo.BootID = fmt.Sprint(time.Now().UnixNano())
	}

	nodeMeta := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: np.name,
			Labels: map[string]string{
				nodeAPIVersionLabelKey:    np.APIVersion,
				nodeRoleLabelKey:          "agent",
				domainLabelKey:            np.ns,
				common.LabelNodeNamespace: np.ns,
				v1.LabelHostname:          np.name,
				v1.LabelOSStable:          nodeInfo.OperatingSystem,
				v1.LabelArchStable:        nodeInfo.Architecture,
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{
					Key:    common.KusciaTaintTolerationKey,
					Value:  "v1",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
			Unschedulable: false,
		},
		Status: v1.NodeStatus{
			NodeInfo: nodeInfo,
			Addresses: []v1.NodeAddress{
				{
					Type:    "InternalIP",
					Address: np.ip,
				},
			},
			Capacity:    np.podsAuditor.Capacity(),
			Allocatable: np.podsAuditor.Allocatable(),
			Conditions:  np.GenerateInitConditions(),
		},
	}

	nlog.Debug("Generate static node info done")
	np.RefreshNodeStatus(ctx, &nodeMeta.Status)
	return nodeMeta
}

// SetAgentReady sets agent ready flag to true.
func (np *GenericNodeProvider) SetAgentReady(ctx context.Context, ready bool, message string) {
	np.isAgentReady = ready
	np.agentReadyMessage = message
}

// GenerateInitConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (np *GenericNodeProvider) GenerateInitConditions() []v1.NodeCondition {
	var conditions []v1.NodeCondition

	// network
	conditions, _ = framework.AddOrUpdateNodeCondition(conditions, v1.NodeCondition{
		Type:    v1.NodeNetworkUnavailable,
		Status:  v1.ConditionFalse,
		Reason:  "RouteCreated",
		Message: "RouteController created a route",
	})
	// PIDPressure
	conditions, _ = framework.AddOrUpdateNodeCondition(conditions, v1.NodeCondition{
		Type:    v1.NodePIDPressure,
		Status:  v1.ConditionFalse,
		Reason:  "AgentHasSufficientPID",
		Message: "Agent has sufficient PID available",
	})

	return conditions
}

// refreshDiskCondition checks whether the disk capacity is under pressure.
// return:
//  1. does disk has pressure? [bool]
//  2. does out of disk? [bool]
//  3. message of disk pressure status [string]
//  4. message of disk out status [string]
func (np *GenericNodeProvider) refreshDiskCondition(name, path string) (bool, bool, string, string) {
	du, err := disk.Usage(path)
	if err != nil {
		nlog.Warnf("Get disk usage info fail, volume=%v, path=%v, err=%v", name, path, err)
		msg := fmt.Sprintf("%v: read disk info fail", name)
		return false, false, msg, msg
	}

	diskPressure := du.UsedPercent >= DiskPressureThreshold || du.InodesUsedPercent >= DiskPressureThreshold
	outOfDisk := du.Free <= DiskOutMinFreeSize || du.InodesFree <= DiskOutMinFreeInode
	// FSType has bug, so we not use it in message
	// du.Used+du.Free=du.Total-reservedBlocks
	pressureMsg := fmt.Sprintf("@%v: space=%v/%v(%.1f%%) inode=%v/%v(%.1f%%)", name,
		math.ByteCountBinary(int64(du.Used)),
		math.ByteCountBinary(int64(du.Used+du.Free)), du.UsedPercent,
		math.ByteCountDecimalRaw(int64(du.InodesUsed)),
		math.ByteCountDecimalRaw(int64(du.InodesTotal)), du.InodesUsedPercent)
	outMsg := fmt.Sprintf("@%v: free_space=%v, free_inode=%v",
		name, math.ByteCountBinary(int64(du.Free)), math.ByteCountDecimalRaw(int64(du.InodesFree)))

	return diskPressure, outOfDisk, pressureMsg, outMsg
}

// RefreshNodeConditions refreshes node condition.
// return: whether the condition changes
func (np *GenericNodeProvider) RefreshNodeConditions(ctx context.Context, st *v1.NodeStatus) bool {
	// agent ready
	var agentChanged bool
	if np.isAgentReady {
		st.Conditions, agentChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeReady,
			Status:  v1.ConditionTrue,
			Reason:  "AgentReady",
			Message: np.agentReadyMessage,
		})
	} else {
		st.Conditions, agentChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeReady,
			Status:  v1.ConditionFalse,
			Reason:  "AgentNotReady",
			Message: np.agentReadyMessage,
		})
	}

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
			st.Conditions, memChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
				Reason: "AgentHasMemoryPressure",
				Message: fmt.Sprintf("Memory is about to run out, total=%v, available=%v",
					math.ByteCountBinary(int64(memory.Total)),
					math.ByteCountBinary(int64(memory.Available))),
			})
		} else {
			st.Conditions, memChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
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
	diskPressure, outOfDisk, pressureMsg, outMsg := np.refreshDiskCondition("agent_volume", np.config.RootDir)
	if diskPressure {
		st.Conditions, diskPressureChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeDiskPressure,
			Status:  v1.ConditionTrue,
			Reason:  "AgentHasDiskPressure",
			Message: fmt.Sprintf("Disk is about to run out. %v", pressureMsg),
		})
	} else {
		st.Conditions, diskPressureChanged = framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
			Type:    v1.NodeDiskPressure,
			Status:  v1.ConditionFalse,
			Reason:  "AgentHasNoDiskPressure",
			Message: fmt.Sprintf("Agent has no disk pressure. %v", pressureMsg),
		})
	}

	if outOfDisk {
		st.Conditions, diskOutChanged =
			framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "OutOfDisk",
				Status:  v1.ConditionTrue,
				Reason:  "AgentIsOutOfDisk",
				Message: fmt.Sprintf("Disk is almost used up. %v", outMsg),
			})
	} else {
		st.Conditions, diskOutChanged =
			framework.AddOrUpdateNodeCondition(st.Conditions, v1.NodeCondition{
				Type:    "OutOfDisk",
				Status:  v1.ConditionFalse,
				Reason:  "AgentHasSufficientDisk",
				Message: fmt.Sprintf("Agent has sufficient disk space available. %v", outMsg),
			})
	}

	return agentChanged || memChanged || diskPressureChanged || diskOutChanged
}

func (np *GenericNodeProvider) RefreshNodeStatus(ctx context.Context, nodeStatus *v1.NodeStatus) bool {
	condChange := np.RefreshNodeConditions(ctx, nodeStatus)

	nlog.Debugf("Refresh node status finish, condition_changed=%v", condChange)
	return condChange
}

// join none empty strings
func joinStrings(elems []string, sep string) string {
	list := make([]string, 0, len(elems))
	for _, elem := range elems {
		if elem != "" {
			list = append(list, elem)
		}
	}
	return strings.Join(list, sep)
}
