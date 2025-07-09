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

//nolint:dupl
package resource

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers/domain/metrics"
	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	maxRetries  = 15
	managerName = "node_resource_manager"
)

const (
	NodeStateReady   = "Ready"
	NodeStateUnReady = "UnReady"
)

const (
	ResourceCheckForAddPod     = "resourceCheckForAddPod"
	ResourceCheckForDeletePod  = "resourceCheckForDeletePod"
	ResourceCheckForUpdatePod  = "ResourceCheckForUpdatePod"
	ResourceCheckForAddNode    = "resourceCheckForAddNode"
	ResourceCheckForDeleteNode = "resourceCheckForDeleteNode"
	ResourceCheckForUpdateNode = "resourceCheckForUpdateNode"
)

type NodeStatusStore struct {
	LocalNodeStatuses map[string]map[string]*LocalNodeStatus
	Lock              sync.RWMutex
}

type LocalNodeStatus struct {
	Status             string                 `json:"status"`
	LastHeartbeatTime  metav1.Time            `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
	UnreadyReason      string                 `json:"unreadyReason,omitempty"`
	TotalCPURequest    int64                  `json:"totalCPURequest"`
	TotalMemRequest    int64                  `json:"totalMemRequest"`
	Allocatable        apicorev1.ResourceList `json:"allocatable"`
}

type NodeResourceManager struct {
	ctx            context.Context
	cancel         context.CancelFunc
	domainInformer kusciaextv1alpha1.DomainInformer
	nodeInformer   informerscorev1.NodeInformer
	podInformer    informerscorev1.PodInformer
	podQueue       workqueue.RateLimitingInterface
	nodeQueue      workqueue.RateLimitingInterface
	resourceStore  *NodeStatusStore
}

func NewNodeResourceManager(
	ctx context.Context,
	domainInformer kusciaextv1alpha1.DomainInformer,
	nodeInformer informerscorev1.NodeInformer,
	podInformer informerscorev1.PodInformer,
	podQueue workqueue.RateLimitingInterface,
	nodeQueue workqueue.RateLimitingInterface) *NodeResourceManager {
	ctx, cancel := context.WithCancel(ctx)
	nodeStatusStore := &NodeStatusStore{
		LocalNodeStatuses: make(map[string]map[string]*LocalNodeStatus),
		Lock:              sync.RWMutex{},
	}

	manager := &NodeResourceManager{
		ctx:            ctx,
		cancel:         cancel,
		domainInformer: domainInformer,
		nodeInformer:   nodeInformer,
		podInformer:    podInformer,
		podQueue:       podQueue,
		nodeQueue:      nodeQueue,
		resourceStore:  nodeStatusStore,
	}

	manager.addPodEventHandler()
	manager.addNodeEventHandler()

	return manager
}

func (nrm *NodeResourceManager) addPodEventHandler() {
	_, _ = nrm.podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			var pod *apicorev1.Pod
			if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				pod, ok = d.Obj.(*apicorev1.Pod)
				if !ok {
					nlog.Errorf("Could not convert deleted final state %v to Pod", d.Obj)
					return false
				}
			} else {
				pod, ok = obj.(*apicorev1.Pod)
				if !ok {
					nlog.Errorf("Could not convert obj %v to Pod", d.Obj)
					return false
				}
			}

			nlog.Debugf("PodInformer EventHandler handle: %s", pod.Name)
			namespace := pod.Namespace
			_, err := nrm.domainInformer.Lister().Get(namespace)
			if err != nil {
				nlog.Errorf("DomainLister get %s failed with %v", namespace, err)
				return false
			}

			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nrm.handlePodAdd,
			UpdateFunc: nrm.handlePodUpdate,
			DeleteFunc: nrm.handlePodDelete,
		},
	})
}

func (nrm *NodeResourceManager) handlePodAdd(obj interface{}) {
	nrm.handlerPod(obj, nil, ResourceCheckForAddPod)
}

func (nrm *NodeResourceManager) handlePodUpdate(newObj, oldObj interface{}) {
	oldPod, _ := oldObj.(*apicorev1.Pod)
	newPod, _ := newObj.(*apicorev1.Pod)

	if !reflect.DeepEqual(oldPod.Spec, newPod.Spec) ||
		newPod.Status.Phase != oldPod.Status.Phase {
		nrm.handlerPod(newPod, oldPod, ResourceCheckForUpdatePod)
	}
}

func (nrm *NodeResourceManager) handlePodDelete(obj interface{}) {
	nrm.handlerPod(obj, nil, ResourceCheckForDeletePod)
}

func (nrm *NodeResourceManager) handlerPod(newObj interface{}, oldObj interface{}, op string) {
	var newPod, oldPod *apicorev1.Pod
	var ok bool

	newPod, ok = newObj.(*apicorev1.Pod)
	if oldObj != nil {
		oldPod, _ = oldObj.(*apicorev1.Pod)
	}

	if !ok {
		nlog.Errorf("Received unexpected object type %v for %s event", newObj, op)
		return
	}

	queueItem := &queue.PodQueueItem{
		NewPod: newPod,
		OldPod: oldPod,
		Op:     op,
	}
	queue.EnqueuePodObject(queueItem, nrm.podQueue)
}

func (nrm *NodeResourceManager) addNodeEventHandler() {
	_, _ = nrm.nodeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			var node *apicorev1.Node
			if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				node, ok = d.Obj.(*apicorev1.Node)
				if !ok {
					nlog.Errorf("Could not convert deleted final state %v to Node", d.Obj)
					return false
				}
			} else {
				node, ok = obj.(*apicorev1.Node)
				if !ok {
					nlog.Errorf("Could not convert obj %v to Node", d.Obj)
					return false
				}
			}

			nlog.Debugf("NodeInformer EventHandler handle: %s", node.Name)
			if !nrm.matchNodeLabels(node) {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nrm.handleNodeAdd,
			UpdateFunc: nrm.handleNodeUpdate,
			DeleteFunc: nrm.handleNodeDelete,
		},
	})
}

func (nrm *NodeResourceManager) handleNodeAdd(obj interface{}) {
	nrm.handlerNode(obj, nil, ResourceCheckForAddNode)
}

func (nrm *NodeResourceManager) handleNodeUpdate(newObj, oldObj interface{}) {
	oldNode, _ := oldObj.(*apicorev1.Node)
	newNode, _ := newObj.(*apicorev1.Node)
	nrm.handlerNode(newNode, oldNode, ResourceCheckForUpdateNode)
}

func (nrm *NodeResourceManager) handleNodeDelete(obj interface{}) {
	nrm.handlerNode(obj, nil, ResourceCheckForDeleteNode)
}

func (nrm *NodeResourceManager) handlerNode(newObj interface{}, oldObj interface{}, op string) {
	var newNode, oldNode *apicorev1.Node
	var ok bool

	newNode, ok = newObj.(*apicorev1.Node)
	if oldObj != nil {
		oldNode, _ = oldObj.(*apicorev1.Node)
	}

	if !ok {
		nlog.Errorf("Received unexpected object type %v for %s event", newObj, op)
		return
	}

	queueItem := &queue.NodeQueueItem{
		NewNode: newNode,
		OldNode: oldNode,
		Op:      op,
	}
	queue.EnqueueNodeObject(queueItem, nrm.nodeQueue)
}

func (nrm *NodeResourceManager) matchNodeLabels(obj *apicorev1.Node) bool {
	if objLabels := obj.GetLabels(); objLabels != nil {
		if domainName, exists := objLabels[common.LabelNodeNamespace]; exists {
			if domainName != "" {
				_, err := nrm.domainInformer.Lister().Get(domainName)
				if err != nil {
					nlog.Errorf("Get domain %s failed with %v", obj.Name, err)
					return false
				}
				return true
			}
			nlog.Errorf("Node %s has empty domain label", obj.Name)
			return false
		}
		nlog.Errorf("Node %s have no label about domain", obj.Name)
		return false
	}
	nlog.Errorf("Node %s get labels failed", obj.Name)
	return false
}

func (nrm *NodeResourceManager) podHandler(item interface{}) error {
	var podItem *queue.PodQueueItem
	if queue.CheckType(item) == "PodQueueItem" {
		podItem = item.(*queue.PodQueueItem)
	} else {
		nlog.Errorf("PodHandler only support PodQueueItem but get : %+v", item)
		return nil
	}

	switch podItem.Op {
	case ResourceCheckForAddPod:
		return nrm.addPodHandler(podItem.NewPod)
	case ResourceCheckForDeletePod:
		return nrm.deletePodHandler(podItem.NewPod)
	case ResourceCheckForUpdatePod:
		return nrm.updatePodHandler(podItem.NewPod, podItem.OldPod)
	default:
		return fmt.Errorf("unknown operation: %s", podItem.Op)
	}
}

func (nrm *NodeResourceManager) addPodHandler(pod *apicorev1.Pod) error {
	nlog.Debugf("NodeResourceManager addPodHandler: %s", pod.Name)

	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	domainName := pod.Namespace
	domainNodes, exists := nrm.resourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when add pod %s", domainName, pod.Name)
		return nil
	}

	nodeName := pod.Spec.NodeName
	if nodeStatus, exists := domainNodes[nodeName]; exists {
		cpu, mem := nrm.calRequestResource(pod)
		nodeStatus.TotalCPURequest += cpu
		nodeStatus.TotalMemRequest += mem

		nlog.Debugf("Added pod %s resources to node %s: CPU=%dm, MEM=%dB",
			pod.Name, nodeName, cpu, mem)
		return nil
	}

	nlog.Warnf("node %s not found when add pod %s", nodeName, pod.Name)
	return nil
}

func (nrm *NodeResourceManager) deletePodHandler(pod *apicorev1.Pod) error {
	nlog.Debugf("NodeResourceManager deletePodHandler: %s", pod.Name)

	domainName := pod.Namespace
	nodeName := pod.Spec.NodeName

	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	domainNodes, exists := nrm.resourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when delete pod %s", domainName, pod.Name)
		return nil
	}

	cpu, mem := nrm.calRequestResource(pod)
	if nodeStatus, exists := domainNodes[nodeName]; exists {
		if nodeStatus.TotalCPURequest >= cpu {
			nodeStatus.TotalCPURequest -= cpu
		} else {
			nodeStatus.TotalCPURequest = 0
		}

		if nodeStatus.TotalMemRequest >= mem {
			nodeStatus.TotalMemRequest -= mem
		} else {
			nodeStatus.TotalMemRequest = 0
		}

		nlog.Debugf("Removed pod %s resources from node %s: CPU=%dm, MEM=%dB",
			pod.Name, nodeName, cpu, mem)
		return nil
	}

	nlog.Warnf("Node %s not found when delete pod %s", nodeName, pod.Name)
	return nil
}

func (nrm *NodeResourceManager) updatePodHandler(newPod, oldPod *apicorev1.Pod) error {
	nlog.Debugf("NodeResourceManager updatePodHandler: newPod=%s, oldPod=%s", newPod.Name, oldPod.Name)

	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	domainName := newPod.Namespace
	domainNodes, exists := nrm.resourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when update pod %s", domainName, newPod.Name)
		return nil
	}

	// Calculate the difference in resources between new and old Pods
	oldCPU, oldMem := nrm.calRequestResource(oldPod)
	newCPU, newMem := nrm.calRequestResource(newPod)
	deltaCPU := newCPU - oldCPU
	deltaMem := newMem - oldMem

	// Dealing with changes to nodeName
	oldNodeName := oldPod.Spec.NodeName
	newNodeName := newPod.Spec.NodeName

	// Scenario 1: Node not changed
	if oldNodeName == newNodeName {
		if nodeStatus, exists := domainNodes[newNodeName]; exists {
			nodeStatus.TotalCPURequest += deltaCPU
			nodeStatus.TotalMemRequest += deltaMem

			if nodeStatus.TotalCPURequest < 0 {
				nodeStatus.TotalCPURequest = 0
			}
			if nodeStatus.TotalMemRequest < 0 {
				nodeStatus.TotalMemRequest = 0
			}

			nlog.Debugf("Updated pod %s resources on node %s: CPU=%+dm, MEM=%+dB",
				newPod.Name, newNodeName, deltaCPU, deltaMem)
			return nil
		}
	}

	// Scenario 2: Node changes occur, Remove resources from old nodes
	if oldNodeStatus, exists := domainNodes[oldNodeName]; exists {
		oldNodeStatus.TotalCPURequest -= oldCPU
		oldNodeStatus.TotalMemRequest -= oldMem

		if oldNodeStatus.TotalCPURequest < 0 {
			oldNodeStatus.TotalCPURequest = 0
		}
		if oldNodeStatus.TotalMemRequest < 0 {
			oldNodeStatus.TotalMemRequest = 0
		}

		nlog.Debugf("Removed old pod %s resources from node %s: CPU=%dm, MEM=%dB",
			newPod.Name, oldNodeName, oldCPU, oldMem)
	}

	// Add resources to a new node
	if newNodeStatus, exists := domainNodes[newNodeName]; exists {
		newNodeStatus.TotalCPURequest += newCPU
		newNodeStatus.TotalMemRequest += newMem

		nlog.Debugf("Added new pod %s resources to node %s: CPU=%dm, MEM=%dB",
			newPod.Name, newNodeName, newCPU, newMem)
		return nil
	}

	return nil
}

func (nrm *NodeResourceManager) nodeHandler(item interface{}) error {
	var nodeItem *queue.NodeQueueItem
	if queue.CheckType(item) == "NodeQueueItem" {
		nodeItem = item.(*queue.NodeQueueItem)
	} else {
		nlog.Errorf("NodeHandler only support NodeQueueItem but get : %+v", item)
		return nil
	}

	switch nodeItem.Op {
	case ResourceCheckForAddNode:
		return nrm.addNodeHandler(nodeItem.NewNode)
	case ResourceCheckForDeleteNode:
		return nrm.deleteNodeHandler(nodeItem.NewNode)
	case ResourceCheckForUpdateNode:
		return nrm.updateNodeHandler(nodeItem.NewNode, nodeItem.OldNode)
	default:
		return fmt.Errorf("unknown operation: %s", nodeItem.Op)
	}
}

func (nrm *NodeResourceManager) Run(workers int) error {
	defer runtime.HandleCrash()
	defer nrm.podQueue.ShutDown()
	defer nrm.nodeQueue.ShutDown()

	nlog.Info("Starting NodeResourceManager workers")
	for i := 0; i < workers; i++ {
		go wait.Until(nrm.runPodHandleWorker, time.Second, nrm.ctx.Done())
		go wait.Until(nrm.runNodeHandleWorker, time.Second, nrm.ctx.Done())
	}

	<-nrm.ctx.Done()
	return nil
}

func (nrm *NodeResourceManager) runPodHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(nrm.ctx, managerName, nrm.podQueue, nrm.podHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(nrm.podQueue.Len()))
	}
}

func (nrm *NodeResourceManager) runNodeHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(nrm.ctx, managerName, nrm.nodeQueue, nrm.nodeHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(nrm.nodeQueue.Len()))
	}
}

func determineNodeStatus(node *apicorev1.Node) (status, reason string) {
	status = NodeStateUnReady
	reason = ""

	for _, cond := range node.Status.Conditions {
		if cond.Type == apicorev1.NodeReady {
			if cond.Status == apicorev1.ConditionTrue {
				status = NodeStateReady
				for _, pressureCond := range node.Status.Conditions {
					if pressureCond.Type == apicorev1.NodeDiskPressure && pressureCond.Status == apicorev1.ConditionTrue {
						status = NodeStateUnReady
						reason = string(apicorev1.NodeDiskPressure)
						break
					}
				}
			} else {
				status = NodeStateUnReady
				reason = string(cond.Type)
			}
			break
		}
	}
	return status, reason
}

func (nrm *NodeResourceManager) addNodeHandler(node *apicorev1.Node) error {
	nlog.Debugf("Adding node %s", node.Name)

	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	domainName := node.Labels[common.LabelNodeNamespace]
	status, reason := determineNodeStatus(node)
	if _, exists := nrm.resourceStore.LocalNodeStatuses[domainName]; !exists {
		nrm.resourceStore.LocalNodeStatuses[domainName] = make(map[string]*LocalNodeStatus)
	}
	nrm.resourceStore.LocalNodeStatuses[domainName][node.Name] = &LocalNodeStatus{
		Status:             status,
		UnreadyReason:      reason,
		TotalCPURequest:    0,
		TotalMemRequest:    0,
		Allocatable:        node.Status.Allocatable,
		LastHeartbeatTime:  metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}

	nlog.Debugf("Added node %s to domain %s, status: %s", node.Name, domainName, status)
	return nil
}

func (nrm *NodeResourceManager) deleteNodeHandler(node *apicorev1.Node) error {
	nlog.Debugf("Deleting node %s", node.Name)

	domainName := node.Labels[common.LabelNodeNamespace]
	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	if nodes, exists := nrm.resourceStore.LocalNodeStatuses[domainName]; exists {
		if _, nodeExists := nodes[node.Name]; nodeExists {
			delete(nodes, node.Name)
			nlog.Debugf("Removed node %s from domain %s", node.Name, domainName)
			return nil
		}
	}

	nlog.Warnf("Node %s not found in domain %s", node.Name, domainName)
	return nil
}

func (nrm *NodeResourceManager) updateNodeHandler(newNode, oldNode *apicorev1.Node) error {
	nlog.Debugf("Updating node %s", newNode.Name)

	nrm.resourceStore.Lock.Lock()
	defer nrm.resourceStore.Lock.Unlock()

	domainName := newNode.Labels[common.LabelNodeNamespace]
	status, reason := determineNodeStatus(newNode)
	allocatable := newNode.Status.Allocatable

	domainNodes := nrm.resourceStore.LocalNodeStatuses[domainName]
	if nodeStatus, exists := domainNodes[newNode.Name]; exists {
		nodeStatus.Status = status
		nodeStatus.UnreadyReason = reason
		nodeStatus.Allocatable = allocatable
		nodeStatus.LastHeartbeatTime = metav1.Now()
		nlog.Debugf("Updated node %s in domain %s, new status: %s node %v", newNode.Name, domainName, status, nodeStatus)
		return nil
	}
	nlog.Warnf("Node %s not found in domain %s during update", newNode.Name, domainName)
	return nil
}

func (nrm *NodeResourceManager) calRequestResource(pod *apicorev1.Pod) (int64, int64) {
	var requestCPURequest, requestMEMRequest int64
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests == nil {
			continue
		}
		if cpu := container.Resources.Requests.Cpu(); cpu != nil {
			requestCPURequest += cpu.MilliValue()
		}
		if mem := container.Resources.Requests.Memory(); mem != nil {
			requestMEMRequest += mem.Value()
		}
	}
	return requestCPURequest, requestMEMRequest
}

func (nrm *NodeResourceManager) ResourceCheck(domainName string, cpuReq, memReq int64) (bool, error) {
	nrm.resourceStore.Lock.RLock()
	defer nrm.resourceStore.Lock.RUnlock()

	domainNodes, exists := nrm.resourceStore.LocalNodeStatuses[domainName]
	if !exists {
		return false, fmt.Errorf("resource-check no node cache to %s", domainName)
	}

	for nodeName, nodeStatus := range domainNodes {
		if nodeStatus.Status != NodeStateReady {
			continue
		}

		nodeCPUValue := nodeStatus.Allocatable.Cpu().MilliValue()
		nodeMEMValue := nodeStatus.Allocatable.Memory().Value()

		nlog.Debugf("Node %s ncv is %d nmv is %d tcr is %d tmr is %d",
			nodeName, nodeCPUValue, nodeMEMValue,
			nodeStatus.TotalCPURequest, nodeStatus.TotalMemRequest)

		if (nodeCPUValue-nodeStatus.TotalCPURequest) >= cpuReq &&
			(nodeMEMValue-nodeStatus.TotalMemRequest) >= memReq {
			nlog.Debugf("Domain %s node %s available resource", domainName, nodeName)
			return true, nil
		}
	}

	return false, fmt.Errorf("resource-check no node status available for domain %s", domainName)
}

func (nrm *NodeResourceManager) Stop() {
	if nrm.cancel != nil {
		nrm.cancel()
	}
}
