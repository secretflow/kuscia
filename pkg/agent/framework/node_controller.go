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
	"fmt"
	"time"

	"k8s.io/utils/pointer"

	coord "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	coordtypes "k8s.io/client-go/kubernetes/typed/coordination/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

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

type NodeGetter interface {
	GetNode() (*corev1.Node, error)
}

// NodeController deals with creating and managing a node object in Kubernetes.
// It can register a node with Kubernetes and periodically update its status.
// NodeController manages a single node entity.
type NodeController struct { // nolint: golint
	nodeProvider NodeProvider
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

	keepNodeOnExit bool
}

// NewNodeController creates a new node controller.
// This does not have any side-effects on the system or kubernetes.
//
// Use the node's `Run` method to register and run the loops to update the node
// in Kubernetes.
//
// Note: When if there are multiple NodeControllerOpts which apply against the same
// underlying options, the last NodeControllerOpt will win.
func NewNodeController(p NodeProvider, nodes v1.NodeInterface, leaseStub coordtypes.LeaseInterface, keepNode bool) (*NodeController, error) {
	node := &NodeController{
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

		keepNodeOnExit: keepNode,
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
func (node *NodeController) Run(ctx context.Context) error {
	defer func() {
		close(node.chStopped)
		nlog.Info("Node controller exited")
	}()

	nlog.Debugf("Starting node controller ...")
	node.nmt = node.nodeProvider.ConfigureNode(ctx)
	node.nodeProvider.SetStatusUpdateCallback(ctx, func(nmt *corev1.Node) {
		node.chStatusUpdate <- nmt
	})

	if err := node.ensureNodeOnStartup(ctx); err != nil {
		return err
	}

	// make lease, must later than ensureNode
	node.lease = &coord.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.nmt.Name,
		},
		Spec: coord.LeaseSpec{
			HolderIdentity:       &node.nmt.Name,
			LeaseDurationSeconds: pointer.Int32(int32(node.pingInterval.Seconds() * 18)), // 3min
			RenewTime:            &metav1.MicroTime{Time: time.Now()},
		},
	}

	if err := node.ensureLease(ctx); err != nil {
		return fmt.Errorf("setup lease no startup fail, detail-> %v", err)
	}

	node.controlLoop(ctx)

	// exit node controller
	// set node condition to NotReady
	// control loop stopped, so cannot use NotifyAgentReady() method
	newList, _ := AddOrUpdateNodeCondition(
		node.nmt.Status.Conditions,
		corev1.NodeCondition{
			Type:    corev1.NodeReady,
			Status:  corev1.ConditionFalse,
			Reason:  "AgentExit",
			Message: "Node not ready due to agent exited",
		},
	)
	node.nmt.Status.Conditions = newList

	if err := node.updateStatus(context.Background()); err != nil {
		nlog.Warnf("Failed to set node %q condition to NotReady: %v", node.nmt.Name, err)
	}

	nlog.Infof("Cordon this node %q ...", node.nmt.Name)
	nodeFromMaster, err := node.nodeStub.Get(context.Background(), node.nmt.Name, emptyGetOptions)
	if err == nil {
		nodeFromMaster.Spec.Unschedulable = true
		if _, err := node.nodeStub.Update(context.Background(), nodeFromMaster, metav1.UpdateOptions{}); err != nil {
			nlog.Warnf("Failed to cordon the node %q: %v", node.nmt.Name, err)
		}
	} else {
		nlog.Warnf("Failed to get node %q: %v", node.nmt.Name, err)
	}

	if node.keepNodeOnExit {
		return nil
	}

	nlog.Info("Unregister this node ...")
	if err := node.nodeStub.Delete(context.Background(), node.nmt.Name, metav1.DeleteOptions{}); err != nil {
		nlog.Warnf("Failed to delete this node: %s", err.Error())
	}
	return nil
}

func (node *NodeController) Stop() chan struct{} {
	close(node.chStopping)
	return node.chStopped
}

// NotifyAgentReady tells k8s master I'm ready.
func (node *NodeController) NotifyAgentReady(ctx context.Context, ready bool, message string) {
	node.nodeProvider.SetAgentReady(ctx, ready, message)
	if node.nodeProvider.RefreshNodeStatus(ctx, &node.nmt.Status) {
		node.chStatusUpdate <- node.nmt
	}
}

func (node *NodeController) GetNode() (*corev1.Node, error) {
	return node.nmt, nil
}

func (node *NodeController) ensureNodeOnStartup(ctx context.Context) error {
	nodeFromMaster, err := node.nodeStub.Get(ctx, node.nmt.Name, emptyGetOptions)
	if err == nil {
		// node already exist, do legal check.
		if getNodeNamespace(nodeFromMaster) != getNodeNamespace(node.nmt) {
			return fmt.Errorf("duplicate node name with other domain, "+
				"please change this node's name, node_name=%v, self_domain=%v",
				node.nmt.Name, getNodeNamespace(node.nmt))
		}

		if isNodeReady(nodeFromMaster) {
			if nodeFromMaster.Status.NodeInfo.MachineID != node.nmt.Status.NodeInfo.MachineID {
				return fmt.Errorf("there is another node with same name but a different machine id is running, please check, node_name=%v", node.nmt.Name)
			}

			nlog.Warnf("There is another node with the same name and machine id is running and will be replaced by the current node, "+
				"node_name=%v, machine_id=%v, another_node_boot_id=%v", node.nmt.Name, node.nmt.Status.NodeInfo.MachineID, nodeFromMaster.Status.NodeInfo.BootID)
		}

		// step 1. update node spec.
		// node updates may only change labels, taints, or capacity
		nodeFromMaster.Labels = node.nmt.Labels
		nodeFromMaster.Annotations = node.nmt.Annotations
		nodeFromMaster.Spec.Taints = node.nmt.Spec.Taints
		nodeFromMaster.Spec.Unschedulable = node.nmt.Spec.Unschedulable
		finalStatus := node.nmt.Status.DeepCopy()
		newNodeSpec, err := node.nodeStub.Update(ctx, nodeFromMaster, metav1.UpdateOptions{})
		if err != nil {
			nlog.Warnf("Failed to update node: %s", err.Error())
			return err
		}

		// step 2. update node status
		node.nmt = newNodeSpec
		node.nmt.Status = *finalStatus
		if err := node.updateStatus(ctx); err != nil {
			nlog.Errorf("Error handling node status update: %v", err)
		}

		return nil
	}

	// nodeStub.Get() fail
	if k8serrors.IsNotFound(err) {
		newNode, err := node.nodeStub.Create(ctx, node.nmt, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error registering node with kubernetes, detail-> %v", err)
		}
		node.nmt = newNode
		return nil
	}

	nlog.Warnf("Failed to get node: %s", err.Error())
	return err
}

// Ready returns a channel that gets closed when the node is fully up and
// running. Note that if there is an error on startup this channel will never
// be started.
func (node *NodeController) Ready() <-chan struct{} {
	return node.chReady
}

func (node *NodeController) controlLoop(ctx context.Context) {
	pingTimer := time.NewTimer(node.pingInterval)
	defer pingTimer.Stop()

	statusNoChangeTimer := time.NewTimer(node.statusNoChangeInterval)
	defer statusNoChangeTimer.Stop()

	statusRefreshTimer := time.NewTimer(node.statusRefreshInterval)
	defer statusRefreshTimer.Stop()

	close(node.chReady)
	nlog.Info("Node controller started")

	for {
		select {
		case <-node.chStopping:
			return // exit
		case updated := <-node.chStatusUpdate:
			// Performing a status update so stop/reset the status update timer in this
			// branch otherwise there could be an unnecessary status update.
			if !statusNoChangeTimer.Stop() {
				<-statusNoChangeTimer.C
			}

			node.nmt.Status = updated.Status
			if err := node.updateStatus(ctx); err != nil {
				nlog.Warnf("Error handling node status update: %v", err)
			}
			statusNoChangeTimer.Reset(node.statusNoChangeInterval)
		case <-statusRefreshTimer.C:
			changed := node.nodeProvider.RefreshNodeStatus(ctx, &node.nmt.Status)
			statusRefreshTimer.Reset(node.statusRefreshInterval)

			if changed {
				if !statusNoChangeTimer.Stop() {
					<-statusNoChangeTimer.C
				}
				if err := node.updateStatus(ctx); err != nil {
					nlog.Warnf("Error handling node status update: %v", err)
				}
				statusNoChangeTimer.Reset(node.statusNoChangeInterval)
			}
		case <-statusNoChangeTimer.C:
			if err := node.updateStatus(ctx); err != nil {
				nlog.Warnf("Error handling node status update: %v", err)
			}
			statusNoChangeTimer.Reset(node.statusNoChangeInterval)
		case <-pingTimer.C:
			if err := node.handlePing(ctx); err != nil {
				nlog.Warnf("Error while handling node ping: %v", err)
			}
			pingTimer.Reset(node.pingInterval)
		}
	}
}

func (node *NodeController) handlePing(ctx context.Context) (retErr error) {
	if err := node.nodeProvider.Ping(ctx); err != nil {
		return fmt.Errorf("error while pinging the node provider, detail-> %v", err)
	}
	return node.updateLease(ctx)
}

func (node *NodeController) updateLease(ctx context.Context) error {
	node.lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}

	l, err := node.leaseStub.Update(ctx, node.lease, metav1.UpdateOptions{})
	if err == nil {
		node.lease = l
		return nil
	}

	// handle err
	if k8serrors.IsNotFound(err) {
		nlog.Warn("Lease not found")
		err = node.ensureLease(ctx)
	}

	// other errors,
	if err != nil {
		nlog.Errorf("Error updating node lease, err-> %v", err)
		// maybe remote lease is updated, now refresh lease for next update
		l, err := node.leaseStub.Get(ctx, node.lease.Name, metav1.GetOptions{})
		if err == nil {
			node.lease = l
		}
		return err
	}

	return nil
}

func (node *NodeController) updateStatus(ctx context.Context) error {
	newNode, err := updateNodeStatus(ctx, node.nodeStub, node.nmt)
	if err != nil {
		if err := node.handleNodeStatusUpdateError(ctx, err); err != nil {
			return err
		}

		newNode, err = updateNodeStatus(ctx, node.nodeStub, node.nmt)
		if err != nil {
			return err
		}
	}

	node.nmt = newNode
	return nil
}

func (node *NodeController) ensureLease(ctx context.Context) error {
	l, err := node.leaseStub.Create(ctx, node.lease, metav1.CreateOptions{})
	if err != nil {
		switch {
		case k8serrors.IsNotFound(err):
			nlog.Warn("Node lease not supported")
			return err
		case k8serrors.IsAlreadyExists(err):
			if err := node.leaseStub.Delete(ctx, node.lease.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
				nlog.Warnf("Could not delete old node lease, lease=%v, err=%v", node.lease.Name, err.Error())
				return fmt.Errorf("old lease exists but could not delete it, detail-> %v", err)
			}
			return node.ensureLease(ctx) // retry
		}
	}

	nlog.Infof("Created new lease, name=%v", l.Name)
	node.lease = l
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

	// Patch the node status to merge other changes on the node.
	return patchNodeStatus(ctx, nodeStub, types.NodeName(n.Name), oldNode, node)
}

func (node *NodeController) handleNodeStatusUpdateError(ctx context.Context, err error) error {
	if !k8serrors.IsNotFound(err) {
		return err
	}

	nlog.Warn("Node not exist")
	newNode := node.nodeProvider.ConfigureNode(ctx)
	newNode.ResourceVersion = ""
	_, err = node.nodeStub.Create(ctx, newNode, metav1.CreateOptions{})
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

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func AddOrUpdateNodeCondition(list []corev1.NodeCondition, newCondition corev1.NodeCondition) ([]corev1.NodeCondition, bool) {
	now := metav1.Now()
	for idx := range list {
		if list[idx].Type == newCondition.Type {
			oldCond := &list[idx]

			changed := oldCond.Status != newCondition.Status ||
				oldCond.Reason != newCondition.Reason || oldCond.Message != newCondition.Message

			oldCond.LastHeartbeatTime = now
			oldCond.Reason = newCondition.Reason
			oldCond.Message = newCondition.Message
			if oldCond.Status != newCondition.Status {
				oldCond.Status = newCondition.Status
				oldCond.LastTransitionTime = now
			}

			return list, changed
		}
	}

	// not found
	newCondition.LastHeartbeatTime = now
	newCondition.LastTransitionTime = now
	return append(list, newCondition), true
}
