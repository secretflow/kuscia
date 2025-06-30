package domain

import (
	"fmt"
	"reflect"
	"sync"

	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	NodeStateReady   = "Ready"
	NodeStateUnReady = "UnReady"
)

var NodeResourceStore = &NodeStatusStore{
	LocalNodeStatuses: make(map[string][]LocalNodeStatus),
	Lock:              sync.RWMutex{},
}

type NodeStatusStore struct {
	LocalNodeStatuses map[string][]LocalNodeStatus
	Lock              sync.RWMutex
}

type LocalNodeStatus struct {
	Name               string                 `json:"name"`
	Status             string                 `json:"status"`
	LastHeartbeatTime  metav1.Time            `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
	UnreadyReason      string                 `json:"unreadyReason,omitempty"`
	TotalCPURequest    int64                  `json:"totalCPURequest"`
	TotalMemRequest    int64                  `json:"totalMemRequest"`
	Allocatable        apicorev1.ResourceList `json:"allocatable"`
}

type NodeResourceManager struct {
	domainInformer   kusciaextv1alpha1.DomainInformer
	nodeInformer     informerscorev1.NodeInformer
	podInformer      informerscorev1.PodInformer
	podQueue         workqueue.RateLimitingInterface
	nodeQueue        workqueue.RateLimitingInterface
}

func NewNodeResourceManager(
	domainInformer kusciaextv1alpha1.DomainInformer,
	nodeInformer informerscorev1.NodeInformer,
	podInformer informerscorev1.PodInformer,
	podQueue workqueue.RateLimitingInterface,
	nodeQueue workqueue.RateLimitingInterface) *NodeResourceManager {
	return &NodeResourceManager{
		domainInformer: domainInformer,
		nodeInformer:   nodeInformer,
		podInformer:    podInformer,
		podQueue:       podQueue,
		nodeQueue:      nodeQueue,
	}
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

			nlog.Infof("PodInformer EventHandler handle: %+v", pod)
			namespace := pod.Namespace
			nodeName := pod.Spec.NodeName
			nlog.Infof("namespace is %s, nodeName is %s", namespace, nodeName)
			domain, err := nrm.domainInformer.Lister().Get(namespace)
			if err != nil {
				nlog.Errorf("DomainLister get %s failed with %v", namespace, err)
				return false
			}

			if domain == nil {
				nlog.Errorf("DomainLister get %s is 0", namespace)
				return false
			}

			if nodeName == "" || pod.ResourceVersion == "" {
				nlog.Errorf("Pod %s/%s has no node assigned or empty resourceVersion, skipping", pod.Namespace, pod.Name)
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
	nlog.Infof("Step handlePodAdd")
	nrm.handlePodCommon(obj, nil, common.ResourceCheckForAddPod)
}

func (nrm *NodeResourceManager) handlePodUpdate(newObj, oldObj interface{}) {
	nlog.Infof("Step handlePodUpdate")
	oldPod, _ := oldObj.(*apicorev1.Pod)
	newPod, _ := newObj.(*apicorev1.Pod)
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	if !reflect.DeepEqual(oldPod.Spec, newPod.Spec) ||
		newPod.Status.Phase != oldPod.Status.Phase {
		nrm.handlePodCommon(newPod, oldPod, common.ResourceCheckForUpdatePod)
	}
}

func (nrm *NodeResourceManager) handlePodDelete(obj interface{}) {
	nlog.Infof("Step handlePodDelete")
	nrm.handlePodCommon(obj, nil, common.ResourceCheckForDeletePod)
}

func (nrm *NodeResourceManager) handlePodCommon(newObj interface{}, oldObj interface{}, op string) {
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

			nlog.Infof("NodeInformer EventHandler handle: %+v", node)
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
	nlog.Infof("Step handleNodeAdd")
	nrm.handleNodeCommon(obj, nil, common.ResourceCheckForAddNode)
}

func (nrm *NodeResourceManager) handleNodeUpdate(newObj, oldObj interface{}) {
	nlog.Infof("Step handleNodeUpdate")
	oldNode, _ := oldObj.(*apicorev1.Node)
	newNode, _ := newObj.(*apicorev1.Node)
	if oldNode.ResourceVersion == newNode.ResourceVersion {
		return
	}

	nrm.handleNodeCommon(newNode, oldNode, common.ResourceCheckForUpdateNode)
}

func (nrm *NodeResourceManager) handleNodeDelete(obj interface{}) {
	nlog.Infof("Step handleNodeDelete")
	nrm.handleNodeCommon(obj, nil, common.ResourceCheckForDeleteNode)
}

func (nrm *NodeResourceManager) handleNodeCommon(newObj interface{}, oldObj interface{}, op string) {
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

func (nrm *NodeResourceManager) initLocalNodeStatus() error {
	nodes, err := nrm.nodeInformer.Lister().List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	if len(nodes) == 0 {
		nlog.Warn("No nodes found in cluster")
		return nil
	}

	var nodeStatuses = make(map[string][]LocalNodeStatus)
	for _, node := range nodes {
		nlog.Infof("node find %s", node.Name)
		domainName, exists := node.Labels[common.LabelNodeNamespace]
		if exists && domainName != "" {
			nlog.Infof("label domainName is %s", domainName)
			_, err := nrm.domainInformer.Lister().Get(domainName)
			if err == nil {
				nlog.Infof("domainInformer find %s", domainName)
				nodeStatus := LocalNodeStatus{
					Name:            node.Name,
					Status:          nodeStatusNotReady,
					UnreadyReason:   "",
					TotalCPURequest: 0,
					TotalMemRequest: 0,
					Allocatable:     node.Status.Allocatable,
				}

				for _, cond := range node.Status.Conditions {
					if cond.Type == apicorev1.NodeReady {
						switch cond.Status {
						case apicorev1.ConditionTrue:
							nodeStatus.Status = NodeStateReady
							for _, cond := range node.Status.Conditions {
								if cond.Type == apicorev1.NodeDiskPressure && cond.Status == apicorev1.ConditionTrue {
									nodeStatus.Status = NodeStateUnReady
									nodeStatus.UnreadyReason = string(apicorev1.NodeDiskPressure)
									break
								}
							}
						default:
							nodeStatus.Status = NodeStateUnReady
							for _, condReason := range node.Status.Conditions {
								if condReason.Status == apicorev1.ConditionTrue {
									nodeStatus.UnreadyReason = string(condReason.Type)
								}
								break
							}
						}
					}
				}

				nodeStatus.LastHeartbeatTime = metav1.Now()
				nodeStatus.LastTransitionTime = metav1.Now()
				if _, exists := nodeStatuses[domainName]; !exists {
					nodeStatuses[domainName] = []LocalNodeStatus{nodeStatus}
				} else {
					nodeStatuses[domainName] = append(nodeStatuses[domainName], nodeStatus)
				}
			}
		}
	}

	nlog.Infof("cal nodeStatuses is %v", nodeStatuses)

	if NodeResourceStore != nil {
		NodeResourceStore.Lock.Lock()
		defer NodeResourceStore.Lock.Unlock()
		NodeResourceStore.LocalNodeStatuses = nodeStatuses
	}

	nlog.Infof("NodeResourceStore.LocalNodeStatuses is %v", NodeResourceStore.LocalNodeStatuses)
	return nil
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
	case common.ResourceCheckForAddPod:
		return nrm.addPodHandler(podItem.NewPod)
	case common.ResourceCheckForDeletePod:
		return nrm.deletePodHandler(podItem.NewPod)
	case common.ResourceCheckForUpdatePod:
		return nrm.updatePodHandler(podItem.NewPod, podItem.OldPod)
	default:
		return fmt.Errorf("unknown operation: %s", podItem.Op)
	}
}

func (nrm *NodeResourceManager) addPodHandler(pod *apicorev1.Pod) error {
	nlog.Infof("Step addPodHandler: %+v", pod)

	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	domainName := pod.Namespace
	nodes, exists := NodeResourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when add pod %s", domainName, pod.Name)
		return nil
	}

	nodeName := pod.Spec.NodeName
	for i := range nodes {
		if nodes[i].Name == nodeName {
			cpu, mem := nrm.calRequestResource(pod)
			nodes[i].TotalCPURequest += cpu
			nodes[i].TotalMemRequest += mem

			NodeResourceStore.LocalNodeStatuses[domainName] = nodes
			nlog.Infof("Added pod %s resources to node %s: CPU=%dm, MEM=%dB",
				pod.Name, nodeName, cpu, mem)
			return nil
		}
	}

	nlog.Warnf("node %s not found when add pod %s", nodeName, pod.Name)
	return nil
}

func (nrm *NodeResourceManager) deletePodHandler(pod *apicorev1.Pod) error {
	nlog.Infof("Step deletePodHandler: %+v", pod)

	domainName := pod.Namespace
	nodeName := pod.Spec.NodeName

	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	nodes, exists := NodeResourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when delete pod %s", domainName, pod.Name)
		return nil
	}

	cpu, mem := nrm.calRequestResource(pod)
	for i := range nodes {
		if nodes[i].Name == nodeName {
			if nodes[i].TotalCPURequest >= cpu {
				nodes[i].TotalCPURequest -= cpu
			} else {
				nodes[i].TotalCPURequest = 0
			}

			if nodes[i].TotalMemRequest >= mem {
				nodes[i].TotalMemRequest -= mem
			} else {
				nodes[i].TotalMemRequest = 0
			}

			NodeResourceStore.LocalNodeStatuses[domainName] = nodes
			nlog.Infof("Removed pod %s resources from node %s: CPU=%dm, MEM=%dB",
				pod.Name, nodeName, cpu, mem)
			return nil
		}
	}

	nlog.Warnf("node %s not found when delete pod %s", nodeName, pod.Name)
	return nil
}

func (nrm *NodeResourceManager) updatePodHandler(newPod, oldPod *apicorev1.Pod) error {
	nlog.Infof("Step updatePodHandler: newPod=%+v, oldPod=%+v", newPod, oldPod)

	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	domainName := newPod.Namespace
	nodes, exists := NodeResourceStore.LocalNodeStatuses[domainName]
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
	if oldNodeName == newNodeName && newNodeName != "" {
		for i := range nodes {
			if nodes[i].Name == newNodeName {
				nodes[i].TotalCPURequest += deltaCPU
				nodes[i].TotalMemRequest += deltaMem

				// Ensure that resources are not negative
				if nodes[i].TotalCPURequest < 0 {
					nodes[i].TotalCPURequest = 0
				}
				if nodes[i].TotalMemRequest < 0 {
					nodes[i].TotalMemRequest = 0
				}

				NodeResourceStore.LocalNodeStatuses[domainName] = nodes
				nlog.Infof("Updated pod %s resources on node %s: CPU=%+dm, MEM=%+dB",
					newPod.Name, newNodeName, deltaCPU, deltaMem)
				return nil
			}
		}
		nlog.Warnf("node %s not found when update pod %s", newNodeName, newPod.Name)
		return nil
	}

	// Scenario 2: Node changes occur
	if oldNodeName != "" {
		// Remove resources from old nodes
		for i := range nodes {
			if nodes[i].Name == oldNodeName {
				nodes[i].TotalCPURequest -= oldCPU
				nodes[i].TotalMemRequest -= oldMem

				if nodes[i].TotalCPURequest < 0 {
					nodes[i].TotalCPURequest = 0
				}
				if nodes[i].TotalMemRequest < 0 {
					nodes[i].TotalMemRequest = 0
				}

				NodeResourceStore.LocalNodeStatuses[domainName] = nodes
				nlog.Infof("Removed old pod %s resources from node %s: CPU=%dm, MEM=%dB",
					newPod.Name, oldNodeName, oldCPU, oldMem)
				break
			}
		}
	}

	// Add resources to a new node
	if newNodeName != "" {
		for i := range nodes {
			if nodes[i].Name == newNodeName {
				nodes[i].TotalCPURequest += newCPU
				nodes[i].TotalMemRequest += newMem

				NodeResourceStore.LocalNodeStatuses[domainName] = nodes
				nlog.Infof("Added new pod %s resources to node %s: CPU=%dm, MEM=%dB",
					newPod.Name, newNodeName, newCPU, newMem)
				return nil
			}
		}
		nlog.Warnf("new node %s not found when update pod %s", newNodeName, newPod.Name)
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
	case common.ResourceCheckForAddNode:
		return nrm.addNodeHandler(nodeItem.NewNode)
	case common.ResourceCheckForDeleteNode:
		return nrm.deleteNodeHandler(nodeItem.NewNode)
	case common.ResourceCheckForUpdateNode:
		return nrm.updateNodeHandler(nodeItem.NewNode, nodeItem.OldNode)
	default:
		return fmt.Errorf("unknown operation: %s", nodeItem.Op)
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
	nlog.Infof("Adding node %s", node.Name)

	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	domainName := node.Labels[common.LabelNodeNamespace]
	status, reason := determineNodeStatus(node)
	nodeStatus := LocalNodeStatus{
		Name:               node.Name,
		Status:             status,
		UnreadyReason:      reason,
		TotalCPURequest:    0,
		TotalMemRequest:    0,
		Allocatable:        node.Status.Allocatable,
		LastHeartbeatTime:  metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}

	if _, exists := NodeResourceStore.LocalNodeStatuses[domainName]; !exists {
		NodeResourceStore.LocalNodeStatuses[domainName] = []LocalNodeStatus{nodeStatus}
	} else {
		NodeResourceStore.LocalNodeStatuses[domainName] = append(NodeResourceStore.LocalNodeStatuses[domainName], nodeStatus)
	}

	nlog.Infof("Added node %s to domain %s, status: %s", node.Name, domainName, status)
	return nil
}

func (nrm *NodeResourceManager) deleteNodeHandler(node *apicorev1.Node) error {
	nlog.Infof("Deleting node %s", node.Name)

	domainName := node.Labels[common.LabelNodeNamespace]
	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	nodes := NodeResourceStore.LocalNodeStatuses[domainName]
	for i := range nodes {
		if nodes[i].Name == node.Name {
			NodeResourceStore.LocalNodeStatuses[domainName] = append(nodes[:i], nodes[i+1:]...)
			nlog.Infof("Removed node %s from domain %s", node.Name, domainName)
			return nil
		}
	}

	nlog.Warnf("Node %s not found in domain %s", node.Name, domainName)
	return nil
}

func (nrm *NodeResourceManager) updateNodeHandler(newNode, oldNode *apicorev1.Node) error {
	nlog.Infof("Updating node %s", newNode.Name)

	NodeResourceStore.Lock.Lock()
	defer NodeResourceStore.Lock.Unlock()

	domainName := newNode.Labels[common.LabelNodeNamespace]
	status, reason := determineNodeStatus(newNode)
	allocatable := newNode.Status.Allocatable

	nodes := NodeResourceStore.LocalNodeStatuses[domainName]
	for i := range nodes {
		if nodes[i].Name == newNode.Name {
			nodes[i].Status = status
			nodes[i].UnreadyReason = reason
			nodes[i].Allocatable = allocatable
			nodes[i].LastHeartbeatTime = metav1.Now()
			nlog.Infof("Updated node %s in domain %s, new status: %s node %v", newNode.Name, domainName, status, nodes[i])
			return nil
		}
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
