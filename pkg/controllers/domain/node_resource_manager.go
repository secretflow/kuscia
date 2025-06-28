package domain

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers/domain/metrics"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	NodeStateReady   = "Ready"
	NodeStateUnReady = "UnReady"
)

const nodeResourceControllerName = "node-resource-controller"

var NodeResourceManager = &NodeStatusManager{
	LocalNodeStatuses: make(map[string][]LocalNodeStatus),
	Lock:              sync.RWMutex{},
}

type NodeStatusManager struct {
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

type ResourceController struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	domainLister          kuscialistersv1alpha1.DomainLister
	nodeLister            listerscorev1.NodeLister
	podLister             listerscorev1.PodLister
	podQueue              workqueue.RateLimitingInterface
	nodeQueue             workqueue.RateLimitingInterface
	cacheSyncs            []cache.InformerSynced
}

func NewResourceController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 5*time.Minute)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	cacheSyncs := []cache.InformerSynced{
		domainInformer.Informer().HasSynced,
		nodeInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
	}

	resourceController := &ResourceController{
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		domainLister:          domainInformer.Lister(),
		nodeLister:            nodeInformer.Lister(),
		podLister:             podInformer.Lister(),
		podQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		nodeQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		cacheSyncs:            cacheSyncs,
	}

	resourceController.ctx, resourceController.cancel = context.WithCancel(ctx)
	resourceController.addPodEventHandler(podInformer)
	resourceController.addNodeEventHandler(nodeInformer)

	nlog.Infof("resourceController init finish %v", resourceController)

	return resourceController
}

func (rc *ResourceController) addPodEventHandler(podInformer informerscorev1.PodInformer) {
	_, _ = podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
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
			_, err := rc.domainLister.Get(namespace)
			if err != nil {
				nlog.Errorf("DomainLister get %s failed with %v", namespace, err)
				return false
			}

			if nodeName == "" || pod.ResourceVersion == "" {
				nlog.Errorf("Pod %s/%s has no node assigned or empty resourceVersion, skipping", pod.Namespace, pod.Name)
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rc.handlePodAdd,
			UpdateFunc: rc.handlePodUpdate,
			DeleteFunc: rc.handlePodDelete,
		},
	})
}

func (rc *ResourceController) handlePodAdd(obj interface{}) {
	nlog.Infof("Step handlePodAdd")
	rc.handlePodCommon(obj, nil, common.ResourceCheckForAddPod)
}

func (rc *ResourceController) handlePodUpdate(newObj, oldObj interface{}) {
	nlog.Infof("Step handlePodUpdate")
	oldPod, _ := oldObj.(*apicorev1.Pod)
	newPod, _ := newObj.(*apicorev1.Pod)
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	if !reflect.DeepEqual(oldPod.Spec, newPod.Spec) ||
		newPod.Status.Phase != oldPod.Status.Phase {
		rc.handlePodCommon(newPod, oldPod, common.ResourceCheckForUpdatePod)
	}
}

func (rc *ResourceController) handlePodDelete(obj interface{}) {
	nlog.Infof("Step handlePodDelete")
	rc.handlePodCommon(obj, nil, common.ResourceCheckForDeletePod)
}

func (rc *ResourceController) handlePodCommon(newObj interface{}, oldObj interface{}, op string) {
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
	queue.EnqueuePodObject(queueItem, rc.podQueue)
}

func (rc *ResourceController) addNodeEventHandler(nodeInformer informerscorev1.NodeInformer) {
	_, _ = nodeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
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
			if !rc.matchNodeLabels(node) {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rc.handleNodeAdd,
			UpdateFunc: rc.handleNodeUpdate,
			DeleteFunc: rc.handleNodeDelete,
		},
	})
}

func (rc *ResourceController) handleNodeAdd(obj interface{}) {
	nlog.Infof("Step handleNodeAdd")
	rc.handleNodeCommon(obj, nil, common.ResourceCheckForAddNode)
}

func (rc *ResourceController) handleNodeUpdate(newObj, oldObj interface{}) {
	nlog.Infof("Step handleNodeUpdate")
	oldNode, _ := oldObj.(*apicorev1.Node)
	newNode, _ := newObj.(*apicorev1.Node)
	if oldNode.ResourceVersion == newNode.ResourceVersion {
		return
	}

	rc.handleNodeCommon(newNode, oldNode, common.ResourceCheckForUpdateNode)
}

func (rc *ResourceController) handleNodeDelete(obj interface{}) {
	nlog.Infof("Step handleNodeDelete")
	rc.handleNodeCommon(obj, nil, common.ResourceCheckForDeleteNode)
}

func (rc *ResourceController) handleNodeCommon(newObj interface{}, oldObj interface{}, op string) {
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
	queue.EnqueueNodeObject(queueItem, rc.nodeQueue)
}

func (rc *ResourceController) matchNodeLabels(obj *apicorev1.Node) bool {
	if objLabels := obj.GetLabels(); objLabels != nil {
		if domainName, exists := objLabels[common.LabelNodeNamespace]; exists {
			if domainName != "" {
				_, err := rc.domainLister.Get(domainName)
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

func (rc *ResourceController) Run(workers int) error {
	defer runtime.HandleCrash()
	defer rc.podQueue.ShutDown()
	defer rc.nodeQueue.ShutDown()

	nlog.Info("Starting nodeResource controller")
	rc.kusciaInformerFactory.Start(rc.ctx.Done())
	rc.kubeInformerFactory.Start(rc.ctx.Done())

	nlog.Info("Waiting for nodeResource informer cache to sync")
	if !cache.WaitForCacheSync(rc.ctx.Done(), rc.cacheSyncs...) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Starting Init LocalNodeStatus")
	err := rc.initLocalNodeStatus()
	if err != nil {
		return fmt.Errorf("failed to initLocalNodeStatus with %v", err)
	}

	nlog.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(rc.runPodHandleWorker, time.Second, rc.ctx.Done())
		go wait.Until(rc.runNodeHandleWorker, time.Second, rc.ctx.Done())
	}

	<-rc.ctx.Done()
	return nil
}

func (rc *ResourceController) initLocalNodeStatus() error {
	nodes, err := rc.nodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	var nodeStatuses = make(map[string][]LocalNodeStatus)
	for _, node := range nodes {
		domainName, exists := node.Labels[common.LabelNodeNamespace]
		if exists && domainName != "" {
			_, err := rc.domainLister.Get(domainName)
			if err == nil {
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

	if NodeResourceManager != nil {
		NodeResourceManager.Lock.Lock()
		defer NodeResourceManager.Lock.Unlock()
		NodeResourceManager.LocalNodeStatuses = nodeStatuses
	}

	nlog.Infof("NodeResourceManager.LocalNodeStatuses is %v", NodeResourceManager.LocalNodeStatuses)
	return nil
}

func (rc *ResourceController) runPodHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), nodeResourceControllerName, rc.podQueue, rc.podHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(rc.podQueue.Len()))
	}
}

func (rc *ResourceController) runNodeHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), nodeResourceControllerName, rc.nodeQueue, rc.nodeHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(rc.nodeQueue.Len()))
	}
}

func (rc *ResourceController) podHandler(item interface{}) error {
	var podItem *queue.PodQueueItem
	if queue.CheckType(item) == "PodQueueItem" {
		podItem = item.(*queue.PodQueueItem)
	} else {
		nlog.Errorf("PodHandler only support PodQueueItem but get : %+v", item)
		return nil
	}

	switch podItem.Op {
	case common.ResourceCheckForAddPod:
		return rc.addPodHandler(podItem.NewPod)
	case common.ResourceCheckForDeletePod:
		return rc.deletePodHandler(podItem.NewPod)
	case common.ResourceCheckForUpdatePod:
		return rc.updatePodHandler(podItem.NewPod, podItem.OldPod)
	default:
		return fmt.Errorf("unknown operation: %s", podItem.Op)
	}
}

func (rc *ResourceController) addPodHandler(pod *apicorev1.Pod) error {
	nlog.Infof("Step addPodHandler: %+v", pod)

	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

	domainName := pod.Namespace
	nodes, exists := NodeResourceManager.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when add pod %s", domainName, pod.Name)
		return nil
	}

	nodeName := pod.Spec.NodeName
	for i := range nodes {
		if nodes[i].Name == nodeName {
			cpu, mem := rc.calRequestResource(pod)
			nodes[i].TotalCPURequest += cpu
			nodes[i].TotalMemRequest += mem

			NodeResourceManager.LocalNodeStatuses[domainName] = nodes
			nlog.Infof("Added pod %s resources to node %s: CPU=%dm, MEM=%dB",
				pod.Name, nodeName, cpu, mem)
			return nil
		}
	}

	nlog.Warnf("node %s not found when add pod %s", nodeName, pod.Name)
	return nil
}

func (rc *ResourceController) deletePodHandler(pod *apicorev1.Pod) error {
	nlog.Infof("Step deletePodHandler: %+v", pod)

	domainName := pod.Namespace
	nodeName := pod.Spec.NodeName

	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

	nodes, exists := NodeResourceManager.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when delete pod %s", domainName, pod.Name)
		return nil
	}

	cpu, mem := rc.calRequestResource(pod)
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

			NodeResourceManager.LocalNodeStatuses[domainName] = nodes
			nlog.Infof("Removed pod %s resources from node %s: CPU=%dm, MEM=%dB",
				pod.Name, nodeName, cpu, mem)
			return nil
		}
	}

	nlog.Warnf("node %s not found when delete pod %s", nodeName, pod.Name)
	return nil
}

func (rc *ResourceController) updatePodHandler(newPod, oldPod *apicorev1.Pod) error {
	nlog.Infof("Step updatePodHandler: newPod=%+v, oldPod=%+v", newPod, oldPod)

	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

	domainName := newPod.Namespace
	nodes, exists := NodeResourceManager.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("Domain %s not found when update pod %s", domainName, newPod.Name)
		return nil
	}

	// Calculate the difference in resources between new and old Pods
	oldCPU, oldMem := rc.calRequestResource(oldPod)
	newCPU, newMem := rc.calRequestResource(newPod)
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

				NodeResourceManager.LocalNodeStatuses[domainName] = nodes
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

				NodeResourceManager.LocalNodeStatuses[domainName] = nodes
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

				NodeResourceManager.LocalNodeStatuses[domainName] = nodes
				nlog.Infof("Added new pod %s resources to node %s: CPU=%dm, MEM=%dB",
					newPod.Name, newNodeName, newCPU, newMem)
				return nil
			}
		}
		nlog.Warnf("new node %s not found when update pod %s", newNodeName, newPod.Name)
	}

	return nil
}

func (rc *ResourceController) nodeHandler(item interface{}) error {
	var nodeItem *queue.NodeQueueItem
	if queue.CheckType(item) == "NodeQueueItem" {
		nodeItem = item.(*queue.NodeQueueItem)
	} else {
		nlog.Errorf("NodeHandler only support NodeQueueItem but get : %+v", item)
		return nil
	}

	switch nodeItem.Op {
	case common.ResourceCheckForAddNode:
		return rc.addNodeHandler(nodeItem.NewNode)
	case common.ResourceCheckForDeleteNode:
		return rc.deleteNodeHandler(nodeItem.NewNode)
	case common.ResourceCheckForUpdateNode:
		return rc.updateNodeHandler(nodeItem.NewNode, nodeItem.OldNode)
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

func (rc *ResourceController) addNodeHandler(node *apicorev1.Node) error {
	nlog.Infof("Adding node %s", node.Name)

	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

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

	if _, exists := NodeResourceManager.LocalNodeStatuses[domainName]; !exists {
		NodeResourceManager.LocalNodeStatuses[domainName] = []LocalNodeStatus{nodeStatus}
	} else {
		NodeResourceManager.LocalNodeStatuses[domainName] = append(NodeResourceManager.LocalNodeStatuses[domainName], nodeStatus)
	}

	nlog.Infof("Added node %s to domain %s, status: %s", node.Name, domainName, status)
	return nil
}

func (rc *ResourceController) deleteNodeHandler(node *apicorev1.Node) error {
	nlog.Infof("Deleting node %s", node.Name)

	domainName := node.Labels[common.LabelNodeNamespace]
	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

	nodes := NodeResourceManager.LocalNodeStatuses[domainName]
	for i := range nodes {
		if nodes[i].Name == node.Name {
			NodeResourceManager.LocalNodeStatuses[domainName] = append(nodes[:i], nodes[i+1:]...)
			nlog.Infof("Removed node %s from domain %s", node.Name, domainName)
			return nil
		}
	}

	nlog.Warnf("Node %s not found in domain %s", node.Name, domainName)
	return nil
}

func (rc *ResourceController) updateNodeHandler(newNode, oldNode *apicorev1.Node) error {
	nlog.Infof("Updating node %s", newNode.Name)

	NodeResourceManager.Lock.Lock()
	defer NodeResourceManager.Lock.Unlock()

	domainName := newNode.Labels[common.LabelNodeNamespace]
	status, reason := determineNodeStatus(newNode)
	allocatable := newNode.Status.Allocatable

	nodes := NodeResourceManager.LocalNodeStatuses[domainName]
	for i := range nodes {
		if nodes[i].Name == newNode.Name {
			nodes[i].Status = status
			nodes[i].UnreadyReason = reason
			nodes[i].Allocatable = allocatable
			nodes[i].LastHeartbeatTime = metav1.Now()

			// Retain the original resource request value
			nodes[i].TotalCPURequest = oldNode.Status.Allocatable.Cpu().MilliValue()
			nodes[i].TotalMemRequest = oldNode.Status.Allocatable.Memory().Value()
			nlog.Infof("Updated node %s in domain %s, new status: %s", newNode.Name, domainName, status)
			return nil
		}
	}

	nlog.Warnf("Node %s not found in domain %s during update", newNode.Name, domainName)
	return nil
}

func (rc *ResourceController) calRequestResource(pod *apicorev1.Pod) (int64, int64) {
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

func (rc *ResourceController) Stop() {
	if rc.cancel != nil {
		rc.cancel()
		rc.cancel = nil
	}
}

func (rc *ResourceController) Name() string {
	return nodeResourceControllerName
}
