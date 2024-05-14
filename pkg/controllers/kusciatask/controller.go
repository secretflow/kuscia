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

// nolint:dulp
package kusciatask

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/handler"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	statusUpdateRetries = 3

	maxBackoffLimit = 3
	controllerName  = "kuscia-task-controller"

	taskQueue       = "task-queue"
	taskDeleteQueue = "task-delete-queue"
)

// Controller is the implementation for KusciaTask resources.
type Controller struct {
	ctx    context.Context
	cancel context.CancelFunc
	// kubeClient is a standard kubernetes clientset
	kubeClient kubernetes.Interface
	// kusciaClient is a clientset for our own API group
	kusciaClient kusciaclientset.Interface

	// Queue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	taskQueue       workqueue.RateLimitingInterface
	taskDeleteQueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// Handler Factory
	handlerFactory *handler.KusciaTaskPhaseHandlerFactory

	// shared informer factory of kubernetes, kuscia
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory

	namespaceLister  corelisters.NamespaceLister
	namespaceSynced  cache.InformerSynced
	podsLister       corelisters.PodLister
	podsSynced       cache.InformerSynced
	servicesSynced   cache.InformerSynced
	servicesLister   corelisters.ServiceLister
	configMapSynced  cache.InformerSynced
	configMapLister  corelisters.ConfigMapLister
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
	kusciaTaskSynced cache.InformerSynced
	appImageSynced   cache.InformerSynced
	trgSynced        cache.InformerSynced
	trgLister        kuscialistersv1alpha1.TaskResourceGroupLister
}

// NewController returns a controller instance.
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	eventRecorder := config.EventRecorder
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 5*time.Minute)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()

	controller := &Controller{
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		namespaceLister:       namespaceInformer.Lister(),
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		podsLister:            podInformer.Lister(),
		podsSynced:            podInformer.Informer().HasSynced,
		servicesSynced:        serviceInformer.Informer().HasSynced,
		servicesLister:        serviceInformer.Lister(),
		configMapSynced:       configMapInformer.Informer().HasSynced,
		configMapLister:       configMapInformer.Lister(),
		kusciaTaskLister:      kusciaTaskInformer.Lister(),
		kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
		appImageSynced:        appImageInformer.Informer().HasSynced,
		trgLister:             trgInformer.Lister(),
		trgSynced:             trgInformer.Informer().HasSynced,
		taskQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskQueue),
		taskDeleteQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskDeleteQueue),
		recorder:              eventRecorder,
	}
	controller.ctx, controller.cancel = context.WithCancel(ctx)
	controller.handlerFactory = handler.NewKusciaTaskPhaseHandlerFactory(&handler.Dependencies{
		KubeClient:       kubeClient,
		KusciaClient:     kusciaClient,
		TrgLister:        trgInformer.Lister(),
		NamespacesLister: namespaceInformer.Lister(),
		PodsLister:       controller.podsLister,
		ServicesLister:   serviceInformer.Lister(),
		ConfigMapLister:  configMapInformer.Lister(),
		AppImagesLister:  appImageInformer.Lister(),
		Recorder:         eventRecorder,
	})

	// kuscia task event handler
	kusciaTaskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueKusciaTask,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueueKusciaTask(newObj)
		},
		DeleteFunc: controller.handleDeletedKusciaTask,
	})

	// task resource group event handler
	trgInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleTaskResourceGroupObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.handleTaskResourceGroupObject(newObj)
		},
		DeleteFunc: controller.handleTaskResourceGroupObject,
	})

	// pod event handler
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handlePodObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			newPod, ok := newObj.(*v1.Pod)
			if !ok {
				nlog.Errorf("Unable convert object to pod")
				return
			}
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				nlog.Errorf("Unable convert object to pod")
				return
			}
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// Pods. Two different versions of the same Pod
				// will always have different RVs.
				return
			}
			controller.handlePodObject(newObj)
		},
		DeleteFunc: controller.handlePodObject,
	})

	// service event handler
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleServiceObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			newService, ok := newObj.(*v1.Service)
			if !ok {
				nlog.Errorf("Unable convert object to service")
				return
			}
			oldService, ok := oldObj.(*v1.Service)
			if !ok {
				nlog.Errorf("Unable convert object to service")
				return
			}
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic re-sync will send update events for all known
				// Services. Two different versions of the same Service
				// will always have different RVs.
				return
			}
			controller.handleServiceObject(newObj)
		},
		DeleteFunc: controller.handleServiceObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the taskQueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer func() {
		c.taskQueue.ShutDown()
		c.taskDeleteQueue.ShutDown()
	}()

	// Start the informer factories to begin populating the informer caches
	nlog.Infof("Starting %v", c.Name())

	c.kusciaInformerFactory.Start(c.ctx.Done())
	c.kubeInformerFactory.Start(c.ctx.Done())

	// Wait for the caches to be synced before starting workers
	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if !cache.WaitForCacheSync(c.ctx.Done(), c.namespaceSynced, c.podsSynced, c.servicesSynced, c.configMapSynced,
		c.kusciaTaskSynced, c.appImageSynced, c.trgSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	// Launch workers to process KusciaTask resources
	for i := 0; i < workers; i++ {
		go c.runWorker()
		go c.runDeletedTaskWorker(c.ctx)
	}

	<-c.ctx.Done()
	nlog.Info("Shutting down workers")

	return nil
}

// Stop the controller.
func (c *Controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// enqueueKusciaTask takes a KusciaTask resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than KusciaTask.
func (c *Controller) enqueueKusciaTask(obj interface{}) {
	var (
		key string
		err error
	)

	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		nlog.Errorf("Error building key of kusciaTask: %v", err)
	}
	c.taskQueue.Add(key)
	nlog.Debugf("Enqueue kusciaTask %q", key)
}

// handleDeletedKusciaTask handles deleted kuscia task.
func (c *Controller) handleDeletedKusciaTask(obj interface{}) {
	kt, ok := obj.(*kusciaapisv1alpha1.KusciaTask)
	if !ok {
		if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = d.Obj
			kt, ok = obj.(*kusciaapisv1alpha1.KusciaTask)
			if !ok {
				return
			}
		}
	}

	c.enqueueDeletedKusciaTask(kt.Name, string(kt.UID))
}

// handleTaskResourceGroupObject enqueue the KusciaTask which the task resource group belongs.
func (c *Controller) handleTaskResourceGroupObject(obj interface{}) {
	var (
		trg *kusciaapisv1alpha1.TaskResourceGroup
		ok  bool
	)

	if trg, ok = obj.(*kusciaapisv1alpha1.TaskResourceGroup); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Errorf("Error decoding object, invalid type %T", obj)
			return
		}
		trg, ok = tombstone.Obj.(*kusciaapisv1alpha1.TaskResourceGroup)
		if !ok {
			nlog.Errorf("Error decoding object tombstone, invalid type %T", tombstone.Obj)
			return
		}
		nlog.Debugf("Recovered deleted object %q from tombstone", trg.Name)
	}

	kt, err := c.kusciaTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(trg.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			kt, err = c.handleNotFoundKusciaTask(trg.Name, trg.Labels)
			if err == nil {
				return
			}
		}
		nlog.Debugf("Get kusciaTask %v failed, %v, skip processing it", trg.Name, err)
		return
	}

	c.enqueueKusciaTask(kt)
}

// handlePodObject enqueue the KusciaTask which the pod belongs.
func (c *Controller) handlePodObject(obj interface{}) {
	var (
		pod *v1.Pod
		ok  bool
	)

	if pod, ok = obj.(*v1.Pod); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Error("Error decoding object, invalid type")
			return
		}
		pod, ok = tombstone.Obj.(*v1.Pod)
		if !ok {
			nlog.Errorf("Error decoding object tombstone, invalid type")
			return
		}
		nlog.Debugf("Recovered deleted object %q from tombstone", pod.Name)
	}

	annotations := pod.Annotations
	if annotations != nil && annotations[common.TaskIDAnnotationKey] != "" {
		taskID := annotations[common.TaskIDAnnotationKey]
		kt, err := c.kusciaTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(taskID)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				kt, err = c.handleNotFoundKusciaTask(taskID, pod.Labels)
				if err == nil {
					return
				}
			}
			nlog.Debugf("Get kusciaTask %v failed, %v, skip processing it", taskID, err)
			return
		}

		c.enqueueKusciaTask(kt)
	}
}

// handleServiceObject enqueue the KusciaTask which the service belongs.
func (c *Controller) handleServiceObject(obj interface{}) {
	var (
		object metav1.Object
		ok     bool
	)

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Error("Error decoding object, invalid type")
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			nlog.Errorf("Error decoding object tombstone, invalid type")
			return
		}
		nlog.Debugf("Recovered deleted object %q from tombstone", object.GetName())
	}

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a pod, we should not do anything more with it.
		if ownerRef.Kind != "Pod" {
			nlog.Debugf("Service %v/%v not belong to this controller, ignore", object.GetNamespace(), object.GetName())
			return
		}
		pod, err := c.podsLister.Pods(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			nlog.Debugf("Ignoring orphaned object %q of pod %q", object.GetSelfLink(), ownerRef.Name)
			return
		}

		if pod.Status.Phase == v1.PodPending {
			return
		}

		c.handlePodObject(pod)
		return
	}
}

func (c *Controller) handleNotFoundKusciaTask(taskID string, labels map[string]string) (*kusciaapisv1alpha1.KusciaTask, error) {
	kt, err := c.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(context.Background(), taskID, metav1.GetOptions{})
	if err == nil {
		return kt, err
	}

	if k8serrors.IsNotFound(err) && labels != nil {
		c.enqueueDeletedKusciaTask(taskID, labels[common.LabelTaskUID])
	}
	return kt, err
}

func (c *Controller) enqueueDeletedKusciaTask(taskName, taskUID string) {
	if taskName == "" || taskUID == "" {
		return
	}
	c.taskDeleteQueue.Add(fmt.Sprintf("%s/%s", taskName, taskUID))
	nlog.Debugf("Enqueue deleted kusciaTask %q", taskName)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the taskQueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
		metrics.WorkerQueueSize.Set(float64(c.taskQueue.Len()))
	}
}

// processNextWorkItem will read a single work item off the taskQueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.taskQueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.taskQueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the taskQueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the taskQueue and attempted again after a back-off
		// period.
		defer c.taskQueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the taskQueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// taskQueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// taskQueue.
		if key, ok = obj.(string); !ok {
			// As the item in the taskQueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.taskQueue.Forget(obj)
			return fmt.Errorf("expected string in taskQueue but got %+v", obj)
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			metrics.TaskRequeueCount.WithLabelValues(key).Inc()
			// Put the item back on the taskQueue to handle any transient errors.
			c.taskQueue.AddRateLimited(key)
			return fmt.Errorf("error handling %q, %v", key, err)
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.taskQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		nlog.Errorf("Failed to process object: %v", err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the KusciaTask resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) (retErr error) {
	startTime := time.Now()

	// Convert the namespace/name string into a distinct namespace and name.
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Invalid resource key: %s", key)
		return nil
	}
	// Get the KusciaTask resource with this namespace/name.
	sharedTask, err := c.kusciaTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(name)
	if err != nil {
		// The KusciaTask resource may no longer exist, in which case we stop processing.
		if k8serrors.IsNotFound(err) {
			metrics.ClearDeadMetrics(key)
			nlog.Infof("KusciaTask %q in work queue may be deleted, skip", key)
			return nil
		}
		return err
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance.
	kusciaTask := sharedTask.DeepCopy()
	// Set default for the new kusciaTask.
	scheme.Scheme.Default(kusciaTask)

	// Return if the task's unschedulable tag is true.
	if kusciaTask.Status.Phase != kusciaapisv1alpha1.TaskFailed &&
		kusciaTask.Labels != nil &&
		kusciaTask.Labels[common.LabelTaskUnschedulable] == common.True {
		nlog.Infof("KusciaTask %q is unschedulable, skipping", key)
		return nil
	}

	// For kusciaTask that is terminating, just return.
	if kusciaTask.DeletionTimestamp != nil {
		nlog.Infof("KusciaTask %q is terminating, skipping", key)
		return nil
	}

	// If task was finished previously, we don't want to redo the termination.
	if kusciaTask.Status.CompletionTime != nil {
		nlog.Infof("KusciaTask %q was finished, skipping", key)
		return nil
	}

	phase := kusciaTask.Status.Phase
	if phase == "" {
		phase = kusciaapisv1alpha1.TaskPending
	}

	// Internal state machine flow.
	needUpdate, err := c.handlerFactory.GetKusciaTaskPhaseHandler(phase).Handle(kusciaTask)
	if err != nil {
		metrics.SyncDurations.WithLabelValues(string(phase), metrics.Failed).Observe(time.Since(startTime).Seconds())
		if c.taskQueue.NumRequeues(key) <= maxBackoffLimit {
			return fmt.Errorf("failed to process kusciaTask %q, %v, retry", key, err)
		}

		c.failKusciaTask(kusciaTask, fmt.Errorf("KusciaTask failed after %vx retry, last error: %v", maxBackoffLimit, err))
		needUpdate = true
	} else {
		metrics.SyncDurations.WithLabelValues(string(phase), metrics.Succeeded).Observe(time.Since(startTime).Seconds())
	}

	if !needUpdate {
		return nil
	}

	// Update kusciatask
	if err = c.updateTaskStatus(sharedTask, kusciaTask); err != nil && !k8serrors.IsConflict(err) {
		return fmt.Errorf("failed to update status for kusciaTask %q, %v", key, err)
	}

	nlog.Infof("Finish syncing KusciaTask %q (%v)", key, time.Since(startTime))

	return nil
}

func (c *Controller) failKusciaTask(kusciaTask *kusciaapisv1alpha1.KusciaTask, err error) {
	now := metav1.Now().Rfc3339Copy()
	kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskFailed
	kusciaTask.Status.Message = err.Error()
	kusciaTask.Status.LastReconcileTime = &now
}

// updateTaskStatus attempts to update the Status.KusciaTask of the given KusciaTask, with a single GET/PUT retry.
func (c *Controller) updateTaskStatus(rawKusciaTask, curKusciaTask *kusciaapisv1alpha1.KusciaTask) (err error) {
	startTime := time.Now()
	defer func() {
		status := metrics.Succeeded
		if err != nil {
			status = metrics.Failed
		}
		metrics.SyncDurations.WithLabelValues("UpdateStatus", status).Observe(time.Since(startTime).Seconds())
	}()

	return utilsres.UpdateKusciaTaskStatus(c.kusciaClient, rawKusciaTask, curKusciaTask)
}

func (c *Controller) runDeletedTaskWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, taskDeleteQueue, c.taskDeleteQueue, c.syncDeletedTaskHandler, 15) {
	}
}

func (c *Controller) syncDeletedTaskHandler(ctx context.Context, key string) error {
	values := strings.Split(key, "/")
	if len(values) != 2 {
		return nil
	}

	taskName := values[0]
	taskUID := values[1]
	pods, _ := c.podsLister.List(labels.SelectorFromSet(labels.Set{common.LabelTaskUID: taskUID}))
	for _, pod := range pods {
		ns := pod.Namespace
		name := pod.Name
		err := c.kubeClient.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to delete pod %v/%v, %v", ns, name, err)
		}
		nlog.Debugf("Delete the pod %v/%v belonging to kusciaTask %v successfully", ns, name, taskName)
	}

	configMaps, _ := c.configMapLister.List(labels.SelectorFromSet(labels.Set{common.LabelTaskUID: taskUID}))
	for _, configMap := range configMaps {
		ns := configMap.Namespace
		name := configMap.Name
		err := c.kubeClient.CoreV1().ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to delete configmap '%s/%s', %v", ns, name, err)
		}
		nlog.Debugf("Delete the configmap %v/%v belonging to kusciaTask %v successfully", ns, name, taskName)
	}

	trg, _ := c.trgLister.Get(taskName)
	if trg != nil {
		if err := c.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Delete(ctx, taskName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete taskResourceGroup %v, %v", taskName, err)
		}
	}

	nlog.Debugf("Finish deleting kusciaTask %q cascaded resources", taskName)
	return nil
}

// Name returns controller name.
func (c *Controller) Name() string {
	return controllerName
}
