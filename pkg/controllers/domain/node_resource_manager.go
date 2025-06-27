package domain

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
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
	nodeStateReady = "Ready"
	nodeStateUnReady = "UnReady"
)

const nodeResourceControllerName = "node-resource-controller"

var NodeResourceManager = &NodeStatusManager{
	localNodeStatuses: make(map[string][]LocalNodeStatus),
	lock:             sync.RWMutex{},
}

var once sync.Once

type NodeStatusManager struct {
	localNodeStatuses map[string][]LocalNodeStatus
	lock     sync.RWMutex
}

type LocalNodeStatus struct {
	Name               string      `json:"name"`
	Status             string      `json:"status"`
	LastHeartbeatTime  metav1.Time `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	UnreadyReason      string      `json:"unreadyReason,omitempty"`
	TotalCPURequest    int64       `json:"totalCPURequest"`
	TotalMemRequest    int64       `json:"totalMemRequest"`
	Allocatable        apicorev1.ResourceList       `json:"allocatable"`
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

	cacheSyncs := []cache.InformerSynced {
		domainInformer.Informer().HasSynced,
		nodeInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
	}

	resourceController := &ResourceController{
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		domainLister:      domainInformer.Lister(),
		nodeLister:        nodeInformer.Lister(),
		podLister:         podInformer.Lister(),
		podQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		nodeQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		cacheSyncs:        cacheSyncs,
	}

	resourceController.ctx, resourceController.cancel = context.WithCancel(ctx)
	resourceController.addPodEventHandler(podInformer)
	resourceController.addNodeEventHandler(nodeInformer)

	return resourceController
}

func (rc *ResourceController) addPodEventHandler(podInformer informerscorev1.PodInformer) {
	_, _ = podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pod, ok := obj.(*apicorev1.Pod)
			nlog.Infof("PodInformer EventHandler handle: %+v", pod)
			if ok {
				namespace := pod.Namespace
				nodeName := pod.Spec.NodeName
				_, err := rc.domainLister.Get(namespace)
				if err != nil {
					nlog.Errorf("DomainLister get %s failed with %v", namespace, err)
					return false
				}

				if nodeName == "" {
					nlog.Errorf("Pod %s/%s has no node assigned yet, skipping", pod.Namespace, pod.Name)
					return false
				}
				return true
			}
			nlog.Errorf("Item %v is not pod type", obj)
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rc.handlePodAdd,
			UpdateFunc: rc.handlePodUpdate,
			DeleteFunc: rc.handlePodDelete,
		},
	})
}

func (rc *ResourceController) handlePodAdd (obj interface{}) {

}

func (rc *ResourceController) handlePodUpdate (newObj, oldObj interface{}) {

}

func (rc *ResourceController) handlePodDelete (obj interface{}) {

}

func (rc *ResourceController) addNodeEventHandler(nodeInformer informerscorev1.NodeInformer) {
	_, _ = nodeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			nodeObj, ok := obj.(*apicorev1.Node)
			nlog.Infof("NodeInformer EventHandler handle: %+v", nodeObj)
			if ok {
				if rc.matchNodeLabels(nodeObj) {
					return true
				}
			}
			nlog.Errorf("Item %v is not node type", obj)
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rc.handleNodeAdd,
			UpdateFunc: rc.handleNodeUpdate,
			DeleteFunc: rc.handleNodeDelete,
		},
	})
}

func (rc *ResourceController) handleNodeAdd (obj interface{}) {

}

func (rc *ResourceController) handleNodeUpdate (newObj, oldObj interface{}) {

}

func (rc *ResourceController) handleNodeDelete (obj interface{}) {

}

func (rc *ResourceController) matchNodeLabels(obj *apicorev1.Node) bool {
	if objLabels := obj.GetLabels(); objLabels != nil {
		if value, exists := objLabels[common.LabelNodeNamespace]; exists {
			if value != "" {
				_, err := rc.domainLister.Get(value)
				if err != nil {
					nlog.Errorf("Get domain %s failed with %v", obj.Name, err)
					return false
				}
				return true
			}
			nlog.Errorf("Node %s hv no domain belonged to", obj.Name)
			return false
		}
		nlog.Errorf("Node %s hv no label about domain", obj.Name)
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
				nodeStatus := LocalNodeStatus {
					Name:               node.Name,
					Status:             nodeStatusNotReady,
					UnreadyReason:      "",
					TotalCPURequest:    0,
					TotalMemRequest:    0,
					Allocatable:        node.Status.Allocatable,
				}

				for _, cond := range node.Status.Conditions {
					if cond.Type == apicorev1.NodeReady {
						switch cond.Status {
						case apicorev1.ConditionTrue:
							nodeStatus.Status = nodeStateReady
							for _, cond := range node.Status.Conditions {
								if cond.Type == apicorev1.NodeDiskPressure && cond.Status == apicorev1.ConditionTrue {
									nodeStatus.Status = nodeStateUnReady
									nodeStatus.UnreadyReason = string(apicorev1.NodeDiskPressure)
									break
								}
							}
						default:
							nodeStatus.Status = nodeStateUnReady
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
		NodeResourceManager.lock.Lock()
		defer NodeResourceManager.lock.Unlock()
		NodeResourceManager.localNodeStatuses = nodeStatuses
	}

	return nil
}

func (rc *ResourceController) runPodHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), controllerName, rc.podQueue, rc.podHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(rc.podQueue.Len()))
	}
}

func (rc *ResourceController) runNodeHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), controllerName, rc.nodeQueue, rc.nodeHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(rc.nodeQueue.Len()))
	}
}

func (rc *ResourceController) podHandler(item interface{}) error {
	var podItem *queue.PodQueueItem
	checkType := queue.CheckType(item)
	if checkType == "PodQueueItem" {
		podItem = item.(*queue.PodQueueItem)
	} else {
		nlog.Errorf("PodHandler only support PodQueueItem but get : %+v", item)
		return nil
	}
	switch podItem.Op {
	case common.ResourceCheckForAddPod:
		return rc.addPodHandler(podItem.Pod)
	case common.ResourceCheckForDeletePod:
		return rc.deletePodHandler(podItem.Pod)
	case common.ResourceCheckForUpdatePod:
		return rc.deletePodHandler(podItem.Pod)
	default:
		return fmt.Errorf("unknown operation: %s", podItem.Op)
	}
}

func (rc *ResourceController) addPodHandler(pod *apicorev1.Pod) error {
	nlog.Debugf("Step addPodHandler: %+v", pod)
	cpuReq, memReq := rc.calRequestResource(pod)
	return rc.nodeStatusManager.AddPodResources(pod.Spec.NodeName, cpuReq, memReq)
}

func (rc *ResourceController) deletePodHandler(pod *apicorev1.Pod) error {
	nlog.Debugf("Step deletePodHandler: %+v", pod)
	cpuReq, memReq := rc.calRequestResource(pod)
	return rc.nodeStatusManager.RemovePodResources(pod.Spec.NodeName, cpuReq, memReq)
}

func (rc *ResourceController) nodeHandler(item interface{}) error {
	var nodeItem *queue.NodeQueueItem
	if queue.CheckType(item) == "NodeQueueItem" {
		nodeItem = item.(*queue.NodeQueueItem)
	} else {
		nlog.Errorf("NodeHandler only support NodeQueueItem but get : %+v", item)
		return nil
	}

	newStatus := LocalNodeStatus{
		Name:       nodeItem.Node.Name,
		DomainName: nodeItem.Node.Labels[common.LabelNodeNamespace],
	}

	for _, cond := range nodeItem.Node.Status.Conditions {
		if cond.Type == apicorev1.NodeReady {
			switch cond.Status {
			case apicorev1.ConditionTrue:
				newStatus.Status = nodeStatusReady
				for _, cond := range nodeItem.Node.Status.Conditions {
					if cond.Type == apicorev1.NodeDiskPressure && cond.Status == apicorev1.ConditionTrue {
						newStatus.Status = nodeStatusNotReady
						newStatus.UnreadyReason = string(apicorev1.NodeDiskPressure)
						break
					}
				}
			default:
				newStatus.Status = nodeStatusNotReady
				for _, condReason := range nodeItem.Node.Status.Conditions {
					if condReason.Status == apicorev1.ConditionTrue {
						newStatus.UnreadyReason = string(condReason.Type)
					}
					break
				}
			}
			newStatus.LastHeartbeatTime = cond.LastHeartbeatTime
			newStatus.LastTransitionTime = cond.LastTransitionTime
			break
		}
	}

	nlog.Debugf("NewStatus to localNodeStatus item is : %+v", newStatus)
	return c.nodeStatusManager.UpdateStatus(newStatus, nodeItem.Op)
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

//func NewNodeStatusManager() *NodeStatusManager {
//	once.Do(func() {
//		nlog.Infof("create nsm instance")
//		instance = &NodeStatusManager{}
//	})
//	return instance
//}
//
//func (m *NodeStatusManager) ReplaceAll(statuses map[string]LocalNodeStatus) {
//	m.statuses = statuses
//}
//
//func (m *NodeStatusManager) GetAll() map[string]LocalNodeStatus {
//	m.lock.RLock()
//	defer m.lock.RUnlock()
//	return m.statuses
//}
//
//func (m *NodeStatusManager) Get(nodeName string) LocalNodeStatus {
//	m.lock.RLock()
//	defer m.lock.RUnlock()
//	return m.statuses[nodeName]
//}
//
//func (m *NodeStatusManager) UpdateStatus(newStatus LocalNodeStatus, op string) error {
//	m.lock.Lock()
//	defer m.lock.Unlock()
//
//	switch op {
//	case common.ResourceCheckForAddNode:
//		m.statuses[newStatus.Name] = newStatus
//	case common.ResourceCheckForUpdateNode:
//		m.statuses[newStatus.Name] = newStatus
//	case common.ResourceCheckForDeleteNode:
//		delete(m.statuses, newStatus.Name)
//	default:
//		return fmt.Errorf("not support type %s", op)
//	}
//	return nil
//}
//
//func (m *NodeStatusManager) AddPodResources(nodeName string, cpu, mem int64) error {
//	m.lock.Lock()
//	defer m.lock.Unlock()
//
//	if status, exists := m.statuses[nodeName]; exists {
//		status.TotalCPURequest += cpu
//		status.TotalMemRequest += mem
//		m.statuses[nodeName] = status
//		return nil
//	}
//	return fmt.Errorf("node %s not found", nodeName)
//}
//
//func (m *NodeStatusManager) RemovePodResources(nodeName string, cpu, mem int64) error {
//	m.lock.Lock()
//	defer m.lock.Unlock()
//
//	if status, exists := m.statuses[nodeName]; exists {
//		status.TotalCPURequest -= cpu
//		status.TotalMemRequest -= mem
//		m.statuses[nodeName] = status
//		return nil
//	}
//	return fmt.Errorf("node %s not found", nodeName)
//}
