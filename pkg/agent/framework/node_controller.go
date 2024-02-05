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

package framework

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/utils"
	"k8s.io/utils/pointer"

	coord "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	coordtypes "k8s.io/client-go/kubernetes/typed/coordination/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/kri"
	"github.com/secretflow/kuscia/pkg/agent/utils/nodeutils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	// The agent creates and then updates its Lease object every 10 seconds (the default update
	// interval). Lease updates occur independently of the NodeStatus updates. If the Lease update
	// fails, the agent retries with exponential backoff starting at 200 milliseconds and capped
	// at 7 seconds.
	// note: backoff not implemented yet.
	defaultPingInterval = 10 * time.Second

	// The agent updates the NodeStatus either when there is change in status,
	// or if there has been no update for a configured interval. The default interval for NodeStatus
	// updates is 5 minutes (much longer than the 40 seconds default timeout for unreachable nodes).
	statusUpdateMaxInterval = 5 * time.Minute

	// Time interval for collecting node status
	defaultStatusRefreshInterval = 30 * time.Second
)

const (
	labelNodeStopTimestamp = "node.kuscia.secretflow/stop-timestamp"

	conditionNodeReused = "NodeReused"
)

var (
	errDuplicateNode = errors.New("duplicate node")
)

type NodeGetter interface {
	GetNode() (*corev1.Node, error)
}

// NodeController deals with creating and managing a node object in Kubernetes.
// It can register a node with Kubernetes and periodically update its status.
// NodeController manages a single node entity.
type NodeController struct { // nolint: golint
	namespace    string
	nodeProvider kri.NodeProvider
	nmt          *corev1.Node
	lease        *coord.Lease

	leaseStub coordtypes.LeaseInterface
	nodeStub  v1.NodeInterface

	pingInterval           time.Duration
	statusNoChangeInterval time.Duration
	statusRefreshInterval  time.Duration

	chStatusUpdate chan *corev1.Node
	chReady        chan struct{}
	chStopping     chan struct{}
	chStopped      chan struct{}
	chAgentReady   chan struct{}

	nodeCfg *config.NodeCfg
}

// NewNodeController creates a new node controller.
// This does not have any side-effects on the system or kubernetes.
//
// Use the node's `Run` method to register and run the loops to update the node
// in Kubernetes.
//
// Note: When if there are multiple NodeControllerOpts which apply against the same
// underlying options, the last NodeControllerOpt will win.
func NewNodeController(namespace string, p kri.NodeProvider, nodes v1.NodeInterface, leaseStub coordtypes.LeaseInterface, cfg *config.NodeCfg) (*NodeController, error) {
	node := &NodeController{
		namespace:    namespace,
		nodeProvider: p,
		nodeStub:     nodes,
		leaseStub:    leaseStub,

		pingInterval:           defaultPingInterval,
		statusNoChangeInterval: statusUpdateMaxInterval,
		statusRefreshInterval:  defaultStatusRefreshInterval,

		chStatusUpdate: make(chan *corev1.Node),
		chReady:        make(chan struct{}),
		chStopping:     make(chan struct{}),
		chStopped:      make(chan struct{}),
		chAgentReady:   make(chan struct{}),

		nodeCfg: cfg,
	}

	return node, nil
}

// Run registers the node in kubernetes and starts loops for updating the node
// status in Kubernetes.
//
// The node status must be updated periodically in Kubernetes to keep the node
// active. Newer versions of Kubernetes support node leases, which are
// essentially light weight pings. Older versions of Kubernetes require updating
// the node status periodically.
//
// If Kubernetes supports node leases this will use leases with a much slower
// node status update (because some things still expect the node to be updated
// periodically), otherwise it will only use node status update with the configured
// ping interval.
func (nc *NodeController) Run(ctx context.Context) error {
	defer func() {
		close(nc.chStopped)
		nlog.Info("Node controller exited")
	}()

	nlog.Debugf("Starting node controller ...")

	reused := false
	if nc.nodeCfg.EnableNodeReuse {
		var err error
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			reused, err = nc.reuseNode(ctx)
			return err
		}); err != nil {
			return fmt.Errorf("failed to reuse node, detail-> %v", err)
		}
	}

	if !reused {
		nlog.Infof("Configure node %s", nc.nodeCfg.NodeName)

		nc.nmt = nc.nodeProvider.ConfigureNode(ctx, nc.nodeCfg.NodeName)

		for {
			if err := nc.ensureNodeOnStartup(ctx); err != nil {
				if err == errDuplicateNode {
					time.Sleep(5 * time.Second)
					continue
				}
				return err
			}
			break
		}
	}

	nc.nodeProvider.SetStatusUpdateCallback(ctx, func(nmt *corev1.Node) {
		nc.chStatusUpdate <- nmt
	})

	// make lease, must later than ensureNode
	nc.lease = &coord.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: nc.nmt.Name,
		},
		Spec: coord.LeaseSpec{
			HolderIdentity:       &nc.nmt.Name,
			LeaseDurationSeconds: pointer.Int32(int32(nc.pingInterval.Seconds() * 18)), // 3min
			RenewTime:            &metav1.MicroTime{Time: time.Now()},
		},
	}

	if err := nc.ensureLease(ctx); err != nil {
		return fmt.Errorf("failed to ensure lease exist, detail-> %v", err)
	}

	close(nc.chReady)
	nlog.Info("Node controller started")

	nc.WaitAgentReady(ctx)
	nlog.Info("Node is ready")

	nc.controlLoop(ctx)

	return nc.exit()
}

// reuseNode will reuse the node if there is a stopped node in master.
func (nc *NodeController) reuseNode(ctx context.Context) (bool, error) {

	// list stopped nodes
	selector := labels.NewSelector()
	stopRequirement, err := labels.NewRequirement(labelNodeStopTimestamp, selection.Exists, []string{})
	if err != nil {
		return false, err
	}
	selector.Add(*stopRequirement)
	domainRequirement, err := labels.NewRequirement(common.LabelNodeNamespace, selection.Equals, []string{nc.namespace})
	if err != nil {
		return false, err
	}
	selector.Add(*domainRequirement)

	stoppedNodeList, err := nc.nodeStub.List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return false, err
	}

	if len(stoppedNodeList.Items) == 0 {
		return false, nil
	}

	reusedNode := &stoppedNodeList.Items[0]

	nlog.Infof("Prepare to reuse node %s, machineID=%v, capacity=%+v", reusedNode.Name, reusedNode.Status.NodeInfo.MachineID, reusedNode.Status.Capacity)

	localNode := nc.nodeProvider.ConfigureNode(ctx, reusedNode.Name)

	// add NodeReused condition
	newList, _ := nodeutils.AddOrUpdateNodeCondition(
		reusedNode.Status.Conditions,
		corev1.NodeCondition{
			Type:   conditionNodeReused,
			Status: corev1.ConditionTrue,
			Reason: "ReuseNode",
			Message: fmt.Sprintf("Node %v reused the node %v, machineID=%v, capacity=%+v", nc.nodeCfg.NodeName,
				reusedNode.Name, reusedNode.Status.NodeInfo.MachineID, reusedNode.Status.Capacity),
		},
	)

	// step 1. update node spec.
	reusedNode.Labels = localNode.Labels
	reusedNode.Annotations = localNode.Annotations
	reusedNode.Spec.Taints = localNode.Spec.Taints
	reusedNode.Spec.Unschedulable = localNode.Spec.Unschedulable

	reusedNode.Status.NodeInfo = localNode.Status.NodeInfo
	reusedNode.Status.Capacity = localNode.Status.Capacity
	reusedNode.Status.Allocatable = localNode.Status.Allocatable
	reusedNode.Status.Addresses = localNode.Status.Addresses
	reusedNode.Status.Conditions = newList

	finalStatus := reusedNode.Status.DeepCopy()
	newNodeSpec, err := nc.nodeStub.Update(ctx, reusedNode, metav1.UpdateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to update node, detail-> %v", err)
	}

	// step 2. update node status
	nc.nmt = newNodeSpec
	nc.nmt.Status = *finalStatus
	if err := nc.updateStatus(ctx); err != nil {
		return false, fmt.Errorf("failed to update node status, detail-> %v", err)
	}

	return true, nil
}

// exit is used to clean up the node controller.
func (nc *NodeController) exit() error {
	if nc.nodeCfg.EnableNodeReuse {
		if nc.nmt.Labels == nil {
			nc.nmt.Labels = make(map[string]string)
		}

		nc.nmt.Labels[labelNodeStopTimestamp] = time.Now().Format(time.RFC3339)

		if err := retry.OnError(retry.DefaultRetry, retriable, func() error {
			_, err := nc.nodeStub.Update(context.Background(), nc.nmt, metav1.UpdateOptions{})
			return err
		}); err != nil {
			nlog.Warnf("Failed to update node %q: %v", nc.nmt.Name, err)
		}

		nlog.Infof("Set node %q label %q", nc.nmt.Name, labelNodeStopTimestamp)
	} else {
		// set node condition to NotReady
		newList, _ := nodeutils.AddOrUpdateNodeCondition(
			nc.nmt.Status.Conditions,
			corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionFalse,
				Reason:  "AgentExit",
				Message: "Node not ready due to agent exited",
			},
		)
		nc.nmt.Status.Conditions = newList
		if err := retry.OnError(retry.DefaultRetry, retriable, func() error {
			return nc.updateStatus(context.Background())
		}); err != nil {
			nlog.Warnf("Failed to update node %q status: %v", nc.nmt.Name, err)
		}

		nlog.Infof("Set node %q status to unready", nc.nmt.Name)
	}

	nlog.Infof("Cordon this node %q ...", nc.nmt.Name)
	nodeFromMaster, err := nc.nodeStub.Get(context.Background(), nc.nmt.Name, emptyGetOptions)
	if err == nil {
		nodeFromMaster.Spec.Unschedulable = true

		if err := retry.OnError(retry.DefaultRetry, retriable, func() error {
			_, err = nc.nodeStub.Update(context.Background(), nodeFromMaster, metav1.UpdateOptions{})
			return err
		}); err != nil {
			nlog.Warnf("Failed to cordon the node %q: %v", nc.nmt.Name, err)
		}
	} else {
		nlog.Warnf("Failed to get node %q: %v", nc.nmt.Name, err)
	}

	if !nc.nodeCfg.EnableNodeReuse && !nc.nodeCfg.KeepNodeOnExit {
		nlog.Info("Unregister this node ...")

		if err := retry.OnError(retry.DefaultRetry, retriable, func() error {
			return nc.nodeStub.Delete(context.Background(), nc.nmt.Name, metav1.DeleteOptions{})
		}); err != nil {
			nlog.Warnf("Failed to delete this node: %v", err)
		}
	}

	return nil
}

func retriable(err error) bool {
	return k8serrors.IsInternalError(err) || k8serrors.IsServiceUnavailable(err) ||
		net.IsConnectionRefused(err)
}

func (nc *NodeController) Stop() {
	close(nc.chStopping)
	<-nc.chStopped
}

// NotifyAgentReady tells k8s master I'm ready.
func (nc *NodeController) NotifyAgentReady() {
	close(nc.chAgentReady)
}

func (nc *NodeController) WaitAgentReady(ctx context.Context) {
	<-nc.chAgentReady

	fillReadyCondition(&nc.nmt.Status)

	if err := nc.updateStatus(ctx); err != nil {
		nlog.Warnf("Error handling node status update: %v", err)
	}
}

func (nc *NodeController) GetNode() (*corev1.Node, error) {
	return nc.nmt, nil
}

func (nc *NodeController) ensureNodeOnStartup(ctx context.Context) error {
	nodeFromMaster, err := nc.nodeStub.Get(ctx, nc.nmt.Name, emptyGetOptions)
	if err == nil {
		// node already exist, do legal check.
		if getNodeNamespace(nodeFromMaster) != getNodeNamespace(nc.nmt) {
			return fmt.Errorf("duplicate node name with other domain, "+
				"please change this node's name, node_name=%v, self_domain=%v",
				nc.nmt.Name, getNodeNamespace(nc.nmt))
		}

		if nodeutils.IsNodeReady(nodeFromMaster) {
			if nodeFromMaster.Status.NodeInfo.MachineID != nc.nmt.Status.NodeInfo.MachineID {
				nlog.Warnf("There is another node with same name but a different machine id is running, please check, node_name=%v", nc.nmt.Name)
				return errDuplicateNode
			}

			nlog.Warnf("There is another node with the same name and machine id is running and will be replaced by the current node, "+
				"node_name=%v, machine_id=%v, another_node_boot_id=%v", nc.nmt.Name, nc.nmt.Status.NodeInfo.MachineID, nodeFromMaster.Status.NodeInfo.BootID)
		}

		// step 1. update node spec.
		// node updates may only change labels, taints, or capacity
		nodeFromMaster.Labels = nc.nmt.Labels
		nodeFromMaster.Annotations = nc.nmt.Annotations
		nodeFromMaster.Spec.Taints = nc.nmt.Spec.Taints
		nodeFromMaster.Spec.Unschedulable = nc.nmt.Spec.Unschedulable
		finalStatus := nc.nmt.Status.DeepCopy()
		newNodeSpec, err := nc.nodeStub.Update(ctx, nodeFromMaster, metav1.UpdateOptions{})
		if err != nil {
			nlog.Warnf("Failed to update node: %s", err.Error())
			return err
		}

		// step 2. update node status
		nc.nmt = newNodeSpec
		nc.nmt.Status = *finalStatus
		if err := nc.updateStatus(ctx); err != nil {
			nlog.Errorf("Error handling node status update: %v", err)
		}

		return nil
	}

	// nodeStub.Get() fail
	if k8serrors.IsNotFound(err) {
		newNode, err := nc.nodeStub.Create(ctx, nc.nmt, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error registering node with kubernetes, detail-> %v", err)
		}
		nc.nmt = newNode
		return nil
	}

	nlog.Warnf("Failed to get node: %s", err.Error())
	return err
}

// Ready returns a channel that gets closed when the node is fully up and
// running. Note that if there is an error on startup this channel will never
// be started.
func (nc *NodeController) Ready() <-chan struct{} {
	return nc.chReady
}

func (nc *NodeController) controlLoop(ctx context.Context) {
	pingTimer := time.NewTimer(nc.pingInterval)
	defer pingTimer.Stop()

	statusNoChangeTimer := time.NewTimer(nc.statusNoChangeInterval)
	defer statusNoChangeTimer.Stop()

	statusRefreshTimer := time.NewTimer(nc.statusRefreshInterval)
	defer statusRefreshTimer.Stop()

	for {
		select {
		case <-nc.chStopping:
			return // exit
		case updated := <-nc.chStatusUpdate:
			// Performing a status update so stop/reset the status update timer in this
			// branch otherwise there could be an unnecessary status update.
			if !statusNoChangeTimer.Stop() {
				<-statusNoChangeTimer.C
			}

			nc.nmt.Status = updated.Status
			if err := nc.updateStatus(ctx); err != nil {
				nlog.Warnf("Error handling node status update: %v", err)
			}
			statusNoChangeTimer.Reset(nc.statusNoChangeInterval)
		case <-statusRefreshTimer.C:
			fillReadyCondition(&nc.nmt.Status)
			changed := nc.nodeProvider.RefreshNodeStatus(ctx, &nc.nmt.Status)
			statusRefreshTimer.Reset(nc.statusRefreshInterval)

			if changed {
				if !statusNoChangeTimer.Stop() {
					<-statusNoChangeTimer.C
				}
				if err := nc.updateStatus(ctx); err != nil {
					nlog.Warnf("Error handling node status update: %v", err)
				}
				statusNoChangeTimer.Reset(nc.statusNoChangeInterval)
			}
		case <-statusNoChangeTimer.C:
			if err := nc.updateStatus(ctx); err != nil {
				nlog.Warnf("Error handling node status update: %v", err)
			}
			statusNoChangeTimer.Reset(nc.statusNoChangeInterval)
		case <-pingTimer.C:
			if err := nc.handlePing(ctx); err != nil {
				nlog.Warnf("Error while handling node ping: %v", err)
			}
			pingTimer.Reset(nc.pingInterval)
		}
	}
}

func (nc *NodeController) handlePing(ctx context.Context) (retErr error) {
	if err := nc.nodeProvider.Ping(ctx); err != nil {
		return fmt.Errorf("error while pinging the node provider, detail-> %v", err)
	}
	return nc.updateLease(ctx)
}

func (nc *NodeController) updateLease(ctx context.Context) error {
	nc.lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}

	l, err := nc.leaseStub.Update(ctx, nc.lease, metav1.UpdateOptions{})
	if err == nil {
		nc.lease = l
		return nil
	}

	// handle err
	if k8serrors.IsNotFound(err) {
		nlog.Warn("Lease not found")
		err = nc.ensureLease(ctx)
	}

	// other errors,
	if err != nil {
		nlog.Errorf("Error updating node lease, err-> %v", err)
		// maybe remote lease is updated, now refresh lease for next update
		l, err := nc.leaseStub.Get(ctx, nc.lease.Name, metav1.GetOptions{})
		if err == nil {
			nc.lease = l
		}
		return err
	}

	return nil
}

func (nc *NodeController) updateStatus(ctx context.Context) error {
	newNode, err := updateNodeStatus(ctx, nc.nodeStub, nc.nmt)
	if err != nil {
		if err := nc.handleNodeStatusUpdateError(ctx, err); err != nil {
			return err
		}

		newNode, err = updateNodeStatus(ctx, nc.nodeStub, nc.nmt)
		if err != nil {
			return err
		}
	}

	nc.nmt = newNode
	return nil
}

func (nc *NodeController) ensureLease(ctx context.Context) error {
	l, err := nc.leaseStub.Create(ctx, nc.lease, metav1.CreateOptions{})
	if err != nil {
		switch {
		case k8serrors.IsNotFound(err):
			nlog.Warn("Node lease not supported")
			return err
		case k8serrors.IsAlreadyExists(err):
			if err := nc.leaseStub.Delete(ctx, nc.lease.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
				nlog.Warnf("Could not delete old node lease, lease=%v, err=%v", nc.lease.Name, err.Error())
				return fmt.Errorf("old lease exists but could not delete it, detail-> %v", err)
			}
			return nc.ensureLease(ctx) // retry
		}
	}

	nlog.Infof("Created new lease, name=%v", l.Name)
	nc.lease = l
	return err
}

// just so we don't have to allocate this on every get request
var emptyGetOptions = metav1.GetOptions{}

// patchNodeStatus patches node status.
func patchNodeStatus(ctx context.Context, nodeStub v1.NodeInterface, nodeName types.NodeName, oldNode *corev1.Node, newNode *corev1.Node) (*corev1.Node, error) {
	patchBytes, err := preparePatchBytesForNodeStatus(nodeName, oldNode, newNode)
	if err != nil {
		return nil, err
	}

	updatedNode, err := nodeStub.Patch(ctx, string(nodeName), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to patch status %q for node %q: %v", patchBytes, nodeName, err)
	}
	return updatedNode, nil
}

func preparePatchBytesForNodeStatus(nodeName types.NodeName, oldNode *corev1.Node, newNode *corev1.Node) ([]byte, error) {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal oldData for node %q: %v", nodeName, err)
	}

	// Reset spec to make sure only patch for Status or ObjectMeta is generated.
	// Note that we don't reset ObjectMeta here, because:
	// 1. This aligns with Nodes().UpdateStatus().
	// 2. Some component does use this to update node annotations.
	newNode.Spec = oldNode.Spec
	newData, err := json.Marshal(newNode)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal newData for node %q: %v", nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return nil, fmt.Errorf("failed to CreateTwoWayMergePatch for node %q: %v", nodeName, err)
	}
	return patchBytes, nil
}

// updateNodeStatus triggers an update to the node status in Kubernetes.
// It first fetches the current node details and then sets the status according
// to the passed in node object.
//
// If you use this function, it is up to you to synchronize this with other operations.
// This reduces the time to second-level precision.
func updateNodeStatus(ctx context.Context, nodeStub v1.NodeInterface, n *corev1.Node) (_ *corev1.Node, retErr error) {
	var node *corev1.Node

	oldNode, err := nodeStub.Get(ctx, n.Name, emptyGetOptions)
	if err != nil {
		return nil, err
	}

	node = oldNode.DeepCopy()
	node.ResourceVersion = ""
	node.Status = n.Status

	checkReadyTransited(&oldNode.Status, &node.Status)

	// Patch the node status to merge other changes on the nc.
	return patchNodeStatus(ctx, nodeStub, types.NodeName(n.Name), oldNode, node)
}

// checkReadyTransited updates LastTransitionTime if the ready status changed.
func checkReadyTransited(oldStatus, newStatus *corev1.NodeStatus) {
	_, oldReadyCond := utils.GetNodeCondition(oldStatus, corev1.NodeReady)
	_, newReadyCond := utils.GetNodeCondition(newStatus, corev1.NodeReady)
	if newReadyCond == nil {
		return
	}

	if oldReadyCond != nil && oldReadyCond.Status == newReadyCond.Status {
		return
	}

	newReadyCond.LastTransitionTime = metav1.Now()
}

func (nc *NodeController) handleNodeStatusUpdateError(ctx context.Context, err error) error {
	if !k8serrors.IsNotFound(err) {
		return err
	}

	nlog.Warn("Node not exist")
	newNode := nc.nodeProvider.ConfigureNode(ctx, nc.nodeCfg.NodeName)
	newNode.ResourceVersion = ""
	_, err = nc.nodeStub.Create(ctx, newNode, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	nlog.Infof("Created new node %s", newNode.Name)
	return nil
}

func getNodeNamespace(node *corev1.Node) string {
	if node.Labels == nil {
		return ""
	}
	return node.Labels[common.LabelNodeNamespace]
}

func fillReadyCondition(status *corev1.NodeStatus) {
	status.Conditions, _ = nodeutils.AddOrUpdateNodeCondition(status.Conditions, corev1.NodeCondition{
		Type:    corev1.NodeReady,
		Status:  corev1.ConditionTrue,
		Reason:  "AgentReady",
		Message: "Agent is ready",
	})
}
