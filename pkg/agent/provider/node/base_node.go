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
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/agent/utils/nodeutils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	apiVersion = "0.26.6"

	labelNodeRole       = "kubernetes.io/role"
	labelNodeAPIVersion = "kubernetes.io/apiVersion"
	labelDomain         = "domain"
	labelRuntime        = "kuscia.secretflow/runtime"
)

type BaseNodeDependence struct {
	Runtime         string
	Namespace       string
	Address         string
	CapacityManager *CapacityManager
}

type BaseNode struct {
	runtime         string
	namespace       string
	address         string
	capacityManager *CapacityManager
}

func newBaseNode(dep *BaseNodeDependence) *BaseNode {
	return &BaseNode{
		runtime:         dep.Runtime,
		namespace:       dep.Namespace,
		address:         dep.Address,
		capacityManager: dep.CapacityManager,
	}
}

func (n *BaseNode) configureCommonNode(ctx context.Context, name string) *v1.Node {
	nlog.Debug("Start generate k8s node")

	nodeInfo := v1.NodeSystemInfo{
		Architecture:    runtime.GOARCH,
		KubeletVersion:  meta.AgentVersionString(),
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
			Name: name,
			Labels: map[string]string{
				labelNodeAPIVersion:       apiVersion,
				labelNodeRole:             "agent",
				labelDomain:               n.namespace,
				labelRuntime:              n.runtime,
				common.LabelNodeNamespace: n.namespace,
				v1.LabelHostname:          name,
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
					Address: n.address,
				},
			},
			Capacity:    n.capacityManager.Capacity(),
			Allocatable: n.capacityManager.Allocatable(),
			Conditions:  n.generateInitConditions(),
		},
	}

	nlog.Debug("Generate static node info done")
	return nodeMeta
}

// GenerateInitConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (n *BaseNode) generateInitConditions() []v1.NodeCondition {
	var conditions []v1.NodeCondition

	// network
	conditions, _ = nodeutils.AddOrUpdateNodeCondition(conditions, v1.NodeCondition{
		Type:    v1.NodeNetworkUnavailable,
		Status:  v1.ConditionFalse,
		Reason:  "RouteCreated",
		Message: "RouteController created a route",
	})
	// PIDPressure
	conditions, _ = nodeutils.AddOrUpdateNodeCondition(conditions, v1.NodeCondition{
		Type:    v1.NodePIDPressure,
		Status:  v1.ConditionFalse,
		Reason:  "AgentHasSufficientPID",
		Message: "Agent has sufficient PID available",
	})

	return conditions
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
