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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	coord "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/kri"
	"github.com/secretflow/kuscia/pkg/agent/utils/nodeutils"
)

// mockNodeProvider is a basic node provider that only uses the passed in context
// on `Ping` to determine if the node is healthy.
type mockNodeProvider struct {
	node *corev1.Node
}

// Ping just implements the NodeProvider interface.
// It returns the error from the passed in context only.
func (mockNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (p mockNodeProvider) ConfigureNode(context.Context, string) *corev1.Node {
	return p.node
}

// This mockNodeProvider does not support updating node status and so this
// function is a no-op.
func (mockNodeProvider) SetStatusUpdateCallback(ctx context.Context, f func(*corev1.Node)) {}

func (mockNodeProvider) RefreshNodeStatus(ctx context.Context, nodeStatus *corev1.NodeStatus) bool {
	return false
}

func TestNodeRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	testNode := testNode(t)
	testP := &testNodeProvider{NodeProvider: &mockNodeProvider{node: testNode}}

	nodes := c.CoreV1().Nodes()
	leases := c.CoordinationV1().Leases(corev1.NamespaceNodeLease)

	// We have to refer to testNodeCopy during the course of the test. testNode is modified by the node controller
	// so it will trigger the race detector.
	testNodeCopy := testNode.DeepCopy()
	ndoeCfg := &config.NodeCfg{}
	node, err := NewNodeController(corev1.NamespaceDefault, testP, nodes, leases, ndoeCfg)
	assert.NoError(t, err)

	interval := 1 * time.Millisecond
	node.pingInterval = interval
	node.statusNoChangeInterval = interval

	chErr := make(chan error)
	defer func() {
		node.Stop()
		assert.NoError(t, <-chErr)
	}()

	go func() {
		chErr <- node.Run(ctx)
		close(chErr)
	}()

	nw := makeWatch(ctx, nodes, testNodeCopy.Name, t)
	defer nw.Stop()
	nr := nw.ResultChan()

	lw := makeWatch(ctx, leases, testNodeCopy.Name, t)
	defer lw.Stop()
	lr := lw.ResultChan()

	var (
		lBefore      *coord.Lease
		nodeUpdates  int
		leaseUpdates int

		iters         = 50
		expectAtLeast = iters / 5
	)

	node.NotifyAgentReady()

	timeout := time.After(30 * time.Second)
	for i := 0; i < iters; i++ {
		var l *coord.Lease

		select {
		case <-timeout:
			t.Fatal("timed out waiting for expected events")
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for event")
			continue
		case err := <-chErr:
			t.Fatal(err) // if this returns at all it is an error regardless if err is nil
		case <-nr:
			nodeUpdates++
			continue
		case le := <-lr:
			l = le.Object.(*coord.Lease)
			leaseUpdates++

			assert.True(t, l.Spec.HolderIdentity != nil)
			assert.NoError(t, err)
			assert.True(t, *l.Spec.HolderIdentity == testNodeCopy.Name)
			if lBefore != nil {
				assert.True(t, lBefore.Spec.RenewTime.Time.Before(l.Spec.RenewTime.Time))
			}

			lBefore = l
		}
	}

	lw.Stop()
	nw.Stop()

	assert.True(t, nodeUpdates >= expectAtLeast)
	assert.True(t, leaseUpdates >= expectAtLeast)

	// trigger an async node status update
	n, err := nodes.Get(ctx, testNode.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	newCondition := corev1.NodeCondition{
		Type:               corev1.NodeConditionType("UPDATED"),
		LastTransitionTime: metav1.Now().Rfc3339Copy(),
	}
	n.Status.Conditions = append(n.Status.Conditions, newCondition)

	nw = makeWatch(ctx, nodes, testNodeCopy.Name, t)
	defer nw.Stop()
	nr = nw.ResultChan()

	testP.triggerStatusUpdate(n)

	eCtx, eCancel := context.WithTimeout(ctx, 10*time.Second)
	defer eCancel()

	select {
	case err := <-chErr:
		t.Fatal(err) // if this returns at all it is an error regardless if err is nil
	case err := <-waitForEvent(eCtx, nr, func(e watch.Event) bool {
		node := e.Object.(*corev1.Node)
		if len(node.Status.Conditions) != len(n.Status.Conditions) {
			return false
		}

		// Check if this is a node update we are looking for
		// Since node updates happen periodically there could be some that occur
		// before the status update that we are looking for happens.
		c := node.Status.Conditions[len(node.Status.Conditions)-1]
		if !c.LastTransitionTime.Equal(&newCondition.LastTransitionTime) {
			return false
		}
		if c.Type != newCondition.Type {
			return false
		}
		return true
	}):
		assert.NoError(t, err, "error waiting for updated node condition")
	}
}

func TestNodeController_RunAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	nodeCfg := &config.NodeCfg{NodeName: "aaa", EnableNodeReuse: true}
	runAndStopTestNodeController(t, c, nodeCfg)

	masterNode, err := c.CoreV1().Nodes().Get(ctx, "aaa", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, masterNode.Spec.Unschedulable, true)
	assert.True(t, nodeutils.IsNodeReady(masterNode))
	assert.True(t, masterNode.Labels[labelNodeStopTimestamp] != "")

	nodeCfg = &config.NodeCfg{NodeName: "bbb", EnableNodeReuse: true, KeepNodeOnExit: false}
	runAndStopTestNodeController(t, c, nodeCfg)
	masterNode, err = c.CoreV1().Nodes().Get(ctx, "aaa", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, masterNode.Spec.Unschedulable, true)
	assert.True(t, nodeutils.IsNodeReady(masterNode))
	assert.True(t, masterNode.Labels[labelNodeStopTimestamp] != "")
	_, err = c.CoreV1().Nodes().Get(ctx, "bbb", metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))

	nodeCfg = &config.NodeCfg{NodeName: "ccc", EnableNodeReuse: false, KeepNodeOnExit: true}
	runAndStopTestNodeController(t, c, nodeCfg)
	masterNode, err = c.CoreV1().Nodes().Get(ctx, "ccc", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, masterNode.Spec.Unschedulable, true)
	assert.False(t, nodeutils.IsNodeReady(masterNode))
	assert.True(t, masterNode.Labels[labelNodeStopTimestamp] == "")

	nodeCfg = &config.NodeCfg{NodeName: "ddd", EnableNodeReuse: false, KeepNodeOnExit: false}
	runAndStopTestNodeController(t, c, nodeCfg)
	masterNode, err = c.CoreV1().Nodes().Get(ctx, "ddd", metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))
}

func runAndStopTestNodeController(t *testing.T, client clientset.Interface, nodeCfg *config.NodeCfg) {
	tNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeCfg.NodeName}}
	testP := &testNodeProvider{NodeProvider: &mockNodeProvider{node: tNode}}

	nodes := client.CoreV1().Nodes()
	leases := client.CoordinationV1().Leases(corev1.NamespaceNodeLease)

	nc, err := NewNodeController(corev1.NamespaceDefault, testP, nodes, leases, nodeCfg)
	assert.NoError(t, err)

	go func() {
		assert.NoError(t, nc.Run(context.Background()))
	}()

	nc.NotifyAgentReady()
	nc.Stop()
}

func TestUpdateNodeStatus(t *testing.T) {
	n := testNode(t)
	n.Status.Conditions = append(n.Status.Conditions, corev1.NodeCondition{
		LastHeartbeatTime: metav1.Now().Rfc3339Copy(),
	})
	n.Status.Phase = corev1.NodePending
	nodes := testclient.NewSimpleClientset().CoreV1().Nodes()

	ctx := context.Background()
	updated, err := updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = nodes.Create(ctx, n, metav1.CreateOptions{})
	assert.NoError(t, err)

	updated, err = updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, n.Status, updated.Status)

	n.Status.Phase = corev1.NodeRunning
	updated, err = updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NoError(t, err)
	assert.Equal(t, n.Status, updated.Status)

	err = nodes.Delete(ctx, n.Name, metav1.DeleteOptions{})
	assert.NoError(t, err)

	_, err = nodes.Get(ctx, n.Name, metav1.GetOptions{})
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = updateNodeStatus(ctx, nodes, updated.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)
}

// TestPingAfterStatusUpdate checks that Ping continues to be called with the specified interval
// after a node status update occurs, when leases are disabled.
//
// Timing ratios used in this test:
// ping interval (10 ms)
// maximum allowed interval = 2.5 * ping interval
// status update interval = 6 * ping interval
//
// The allowed maximum time is 2.5 times the ping interval because
// the status update resets the ping interval timer, meaning
// that there can be a full two interval durations between
// successive calls to Ping. The extra half is to allow
// for timing variations when using such short durations.
//
// Once the node controller is ready:
// send status update after 10 * ping interval
// end test after another 10 * ping interval
func TestPingAfterStatusUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()
	nodes := c.CoreV1().Nodes()
	leases := c.CoordinationV1().Leases(corev1.NamespaceNodeLease)

	testNode := testNode(t)
	testP := &testNodeProviderPing{testNodeProvider: testNodeProvider{
		NodeProvider: &mockNodeProvider{node: testNode},
	}}

	// We have to refer to testNodeCopy during the course of the test. testNode is modified by the node controller
	// so it will trigger the race detector.
	testNodeCopy := testNode.DeepCopy()
	cfgNode := &config.NodeCfg{}
	node, err := NewNodeController(corev1.NamespaceDefault, testP, nodes, leases, cfgNode)
	assert.NoError(t, err)
	node.nmt = testNode

	interval := 10 * time.Millisecond
	// 100ms to make UT stable
	maxAllowedInterval := time.Duration(10 * float64(interval.Nanoseconds()))
	node.pingInterval = interval
	node.statusNoChangeInterval = interval

	chErr := make(chan error, 1)
	go func() {
		chErr <- node.Run(ctx)
	}()
	node.NotifyAgentReady()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	// wait for the node to be ready
	select {
	case <-timer.C:
		t.Fatal("timeout waiting for node to be ready")
	case <-chErr:
		t.Fatalf("node.Run returned earlier than expected: %v", err)
	case <-node.Ready():
	}

	notifyTimer := time.After(interval * time.Duration(10))
	select {
	case <-notifyTimer:
		testP.triggerStatusUpdate(testNodeCopy)
	}

	endTimer := time.After(interval * time.Duration(10))
	select {
	case <-endTimer:
		break
	}

	assert.True(t, testP.maxPingInterval < maxAllowedInterval,
		"maximum time between node pings (%v) was greater than the maximum expected interval (%v)",
		testP.maxPingInterval, maxAllowedInterval)
}

func TestDuplicateNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	nodeCfg := &config.NodeCfg{NodeName: "aaa"}
	nodes := c.CoreV1().Nodes()

	preNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeCfg.NodeName}}
	preNode.Status.NodeInfo.MachineID = "aaaaaa"
	preNode.Status.Conditions, _ = nodeutils.AddOrUpdateNodeCondition(preNode.Status.Conditions, corev1.NodeCondition{
		Type:    corev1.NodeReady,
		Status:  corev1.ConditionTrue,
		Reason:  "AgentReady",
		Message: "Agent is ready",
	})
	var err error
	preNode, err = nodes.Create(ctx, preNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	tNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeCfg.NodeName}}
	tNode.Status.NodeInfo.MachineID = "bbbbbb"
	testP := &testNodeProvider{NodeProvider: &mockNodeProvider{node: tNode}}
	leases := c.CoordinationV1().Leases(corev1.NamespaceNodeLease)
	nc, err := NewNodeController(corev1.NamespaceDefault, testP, nodes, leases, nodeCfg)
	assert.NoError(t, err)

	go func() {
		assert.NoError(t, nc.Run(context.Background()))
	}()

	time.Sleep(2 * time.Second)
	preNode.Status.Conditions, _ = nodeutils.AddOrUpdateNodeCondition(nc.nmt.Status.Conditions, corev1.NodeCondition{
		Type:    corev1.NodeReady,
		Status:  corev1.ConditionFalse,
		Reason:  "AgentNotReady",
		Message: "Agent is not ready",
	})
	_, err = nodes.UpdateStatus(ctx, preNode, metav1.UpdateOptions{})
	assert.NoError(t, err)

	select {
	case <-time.After(time.Second * 10):
		assert.Fail(t, "timeout")
	case <-nc.Ready():
		break
	}

	nc.NotifyAgentReady()
	nc.Stop()
}

func testNode(t *testing.T) *corev1.Node {
	n := &corev1.Node{}
	n.Name = strings.ToLower(t.Name())
	return n
}

type testNodeProvider struct {
	kri.NodeProvider
	statusHandlers []func(*corev1.Node)
}

func (p *testNodeProvider) SetStatusUpdateCallback(ctx context.Context, h func(*corev1.Node)) {
	p.statusHandlers = append(p.statusHandlers, h)
}

func (p *testNodeProvider) SetAgentReady(ctx context.Context, ready bool, message string) {}

func (p *testNodeProvider) triggerStatusUpdate(n *corev1.Node) {
	for _, h := range p.statusHandlers {
		h(n)
	}
}

// testNodeProviderPing tracks the maximum time interval between calls to Ping
type testNodeProviderPing struct {
	testNodeProvider
	lastPingTime    time.Time
	maxPingInterval time.Duration
}

func (tnp *testNodeProviderPing) Ping(ctx context.Context) error {
	now := time.Now()
	if tnp.lastPingTime.IsZero() {
		tnp.lastPingTime = now
		return nil
	}
	if now.Sub(tnp.lastPingTime) > tnp.maxPingInterval {
		tnp.maxPingInterval = now.Sub(tnp.lastPingTime)
	}
	tnp.lastPingTime = now
	return nil
}

type watchGetter interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

func makeWatch(ctx context.Context, wc watchGetter, name string, t *testing.T) watch.Interface {
	t.Helper()

	w, err := wc.Watch(ctx, metav1.ListOptions{FieldSelector: "name=" + name})
	assert.NoError(t, err)
	return w
}

// waitForEvent waits for the `check` function to return true
// `check` is run when an event is received
// Cancelling the context will cancel the wait, with the context error sent on
// the returned channel.
func waitForEvent(ctx context.Context, chEvent <-chan watch.Event, check func(watch.Event) bool) <-chan error {
	chErr := make(chan error, 1)
	go func() {

		for {
			select {
			case e := <-chEvent:
				if check(e) {
					chErr <- nil
					return
				}
			case <-ctx.Done():
				chErr <- ctx.Err()
				return
			}
		}
	}()

	return chErr
}
