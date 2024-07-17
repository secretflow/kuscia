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
package taskresourcegroup

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/taskresourcegroup/handler"
	"github.com/secretflow/kuscia/pkg/controllers/taskresourcegroup/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	// maxRetries is the number of times a object will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a object is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15

	defaultTaskResourceGroupLifecycleSeconds     = 300
	defaultTaskResourceGroupRetryDurationSeconds = 30

	statusUpdateRetries = 3
)

const (
	controllerName            = "taskresourcegroup-controller"
	trgReserveFailedQueueName = "taskresourcegroup-reserve-failed-queue"
	trgLifecycleQueueName     = "taskresourcegroup-lifecycle-queue"
)

// Controller is the implementation for managing resources.
type Controller struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kubeInformerFactory   informers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	namespaceLister       listers.NamespaceLister
	namespaceSynced       cache.InformerSynced
	podLister             listers.PodLister
	podSynced             cache.InformerSynced
	trLister              kuscialistersv1alpha1.TaskResourceLister
	trSynced              cache.InformerSynced
	trgSynced             cache.InformerSynced
	trgLister             kuscialistersv1alpha1.TaskResourceGroupLister
	trgQueue              workqueue.RateLimitingInterface
	trgReserveFailedQueue workqueue.DelayingInterface
	trgLifecycleQueue     workqueue.DelayingInterface
	recorder              record.EventRecorder
	handlerFactory        *handler.TaskResourceGroupPhaseHandlerFactory
}

// NewController returns a controller instance.
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	eventRecorder := config.EventRecorder
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 5*time.Minute)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()

	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	controller := &Controller{
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		namespaceLister:       nsInformer.Lister(),
		namespaceSynced:       nsInformer.Informer().HasSynced,
		podLister:             podInformer.Lister(),
		podSynced:             podInformer.Informer().HasSynced,
		trLister:              trInformer.Lister(),
		trSynced:              trInformer.Informer().HasSynced,
		trgLister:             trgInformer.Lister(),
		trgSynced:             trgInformer.Informer().HasSynced,
		trgQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		trgReserveFailedQueue: workqueue.NewNamedDelayingQueue(trgReserveFailedQueueName),
		trgLifecycleQueue:     workqueue.NewNamedDelayingQueue(trgLifecycleQueueName),
		recorder:              eventRecorder,
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)
	controller.handlerFactory = handler.NewTaskResourceGroupPhaseHandlerFactory(&handler.Dependencies{
		KubeClient:      controller.kubeClient,
		KusciaClient:    controller.kusciaClient,
		NamespaceLister: controller.namespaceLister,
		PodLister:       controller.podLister,
		TrLister:        controller.trLister,
	})

	trgInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAddedTaskResourceGroup,
		UpdateFunc: controller.handleUpdatedTaskResourceGroup,
		DeleteFunc: controller.handleDeletedTaskResourceGroup,
	})

	trInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedOrDeletedTaskResource,
			UpdateFunc: controller.handleUpdatedTaskResource,
			DeleteFunc: controller.handleAddedOrDeletedTaskResource,
		},
	})

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: controller.handleAddedPod,
		},
	})

	return controller
}

// resourceFilter is used to filter resource.
func (c *Controller) resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch t := obj.(type) {
		case *corev1.Pod:
			return c.matchLabels(t)
		case *kusciaapisv1alpha1.TaskResource:
			return c.matchLabels(t)
		default:
			return false
		}
	}

	rs, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		return filter(rs.Obj)
	}

	return filter(obj)
}

// matchLabels is used to match obj labels.
func (c *Controller) matchLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	if _, ok := labels[common.LabelResourceVersionUnderHostCluster]; ok {
		return false
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}

	switch obj.(type) {
	case *corev1.Pod:
		if annotations[kusciaapisv1alpha1.TaskResourceKey] != "" &&
			labels[kusciaapisv1alpha1.TaskResourceUID] != "" {
			return true
		}
	case *kusciaapisv1alpha1.TaskResource:
		if annotations[common.TaskResourceGroupAnnotationKey] != "" &&
			labels[common.LabelTaskResourceGroupUID] != "" {
			return true
		}
	default:
		return false
	}
	return false
}

// handleAddedTaskResourceGroup is used to handle added task resource.
func (c *Controller) handleAddedTaskResourceGroup(obj interface{}) {
	trg, ok := obj.(*kusciaapisv1alpha1.TaskResourceGroup)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResourceGroup", obj)
		return
	}

	if trg.Status.Phase != kusciaapisv1alpha1.TaskResourceGroupPhaseReserved &&
		trg.Status.Phase != kusciaapisv1alpha1.TaskResourceGroupPhaseFailed &&
		trg.DeletionTimestamp == nil {
		lifecycleSeconds := trg.Spec.LifecycleSeconds
		if lifecycleSeconds == 0 {
			lifecycleSeconds = defaultTaskResourceGroupLifecycleSeconds
		}
		c.trgLifecycleQueue.AddAfter(trg.Name, time.Duration(lifecycleSeconds)*time.Second)
	}

	queue.EnqueueObjectWithKey(obj, c.trgQueue)
}

// handleUpdatedTaskResourceGroup is used to handle updated task resource.
func (c *Controller) handleUpdatedTaskResourceGroup(oldObj, newObj interface{}) {
	oldTrg, ok := oldObj.(*kusciaapisv1alpha1.TaskResourceGroup)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResourceGroup", oldObj)
		return
	}

	newTrg, ok := newObj.(*kusciaapisv1alpha1.TaskResourceGroup)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResourceGroup", newObj)
		return
	}

	if newTrg.ResourceVersion == oldTrg.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newTrg, c.trgQueue)
}

// handleDeletedTaskResourceGroup is used to handle deleted task resource.
func (c *Controller) handleDeletedTaskResourceGroup(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.trgQueue)
}

// handleAddedOrDeletedTaskResource is used to handle added or deleted task resource.
func (c *Controller) handleAddedOrDeletedTaskResource(obj interface{}) {
	tr, ok := obj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Warnf("couldn't get object from tombstone %#v", obj)
			return
		}
		tr, ok = tombstone.Obj.(*kusciaapisv1alpha1.TaskResource)
		if !ok {
			nlog.Warnf("tombstone contained object that is not a TaskResource %#v", obj)
			return
		}
	}

	c.trgQueue.Add(tr.Annotations[common.TaskResourceGroupAnnotationKey])
}

// handleUpdatedTaskResource is used to handle updated task resource.
func (c *Controller) handleUpdatedTaskResource(oldObj, newObj interface{}) {
	oldTr, ok := oldObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResource", oldObj)
		return
	}

	newTr, ok := newObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResource", newObj)
		return
	}

	if newTr.ResourceVersion == oldTr.ResourceVersion {
		return
	}

	c.trgQueue.Add(newTr.Annotations[common.TaskResourceGroupAnnotationKey])
}

// handleAddedPod is used to handle added pod.
func (c *Controller) handleAddedPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		nlog.Warnf("Object %#v is not a Pod", obj)
		return
	}

	trName := pod.Annotations[kusciaapisv1alpha1.TaskResourceKey]
	tr, err := c.trLister.TaskResources(pod.Namespace).Get(trName)
	if err != nil {
		nlog.Warnf("Get task resource %v failed, %v", trName, err)
		return
	}

	trgName := tr.Annotations[common.TaskResourceGroupAnnotationKey]
	if trgName != "" {
		c.trgQueue.Add(trgName)
	}
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the work queue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer func() {
		c.trgQueue.ShutDown()
		c.trgReserveFailedQueue.ShutDown()
		c.trgLifecycleQueue.ShutDown()
	}()

	nlog.Infof("Starting %v", c.Name())
	c.kubeInformerFactory.Start(c.ctx.Done())
	c.kusciaInformerFactory.Start(c.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.namespaceSynced, c.podSynced, c.trSynced, c.trgSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(c.ctx, c.runWorker, time.Second)
		go wait.Until(c.handleExpiredTrg, time.Second, c.ctx.Done())
		go wait.Until(c.handleReserveFailedTrg, time.Second, c.ctx.Done())
	}

	<-c.ctx.Done()
	return nil
}

// Stop is used to stop the controller.
func (c *Controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, controllerName, c.trgQueue, c.syncHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(c.trgQueue.Len()))
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) (err error) {
	rawTrg, err := c.trgLister.Get(key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Task resource group %v maybe deleted, skip to handle it", key)
			return nil
		}
		return err
	}

	if rawTrg.DeletionTimestamp != nil {
		nlog.Infof("Task resource group %v is terminating, skip to handle it", key)
		return nil
	}

	if rawTrg.Status.CompletionTime != nil {
		nlog.Infof("TaskResourceGroup %q was completed, skip to handle it", key)
		return nil
	}

	trg := rawTrg.DeepCopy()
	if c.needHandleExpiredTrg(trg) {
		return nil
	}

	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed && !c.needHandleReserveFailedTrg(trg) {
		return nil
	}

	phase := trg.Status.Phase
	if phase == "" {
		phase = kusciaapisv1alpha1.TaskResourceGroupPhasePending
	}

	if c.skipProcessTrg(trg, phase) {
		return nil
	}

	needUpdate, err := c.handlerFactory.GetTaskResourceGroupPhaseHandler(phase).Handle(trg)
	if err != nil {
		if c.trgQueue.NumRequeues(key) < maxRetries {
			return err
		}

		failTaskResourceGroup(trg)
		needUpdate = true
	}

	if needUpdate {
		err = c.updateTaskResourceGroupStatus(ctx, rawTrg, trg)
	}

	return err
}

func failTaskResourceGroup(trg *kusciaapisv1alpha1.TaskResourceGroup) {
	now := metav1.Now()
	trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
	trg.Status.LastTransitionTime = &now
	trg.Status.CompletionTime = &now
}

func (c *Controller) skipProcessTrg(trg *kusciaapisv1alpha1.TaskResourceGroup, phase kusciaapisv1alpha1.TaskResourceGroupPhase) bool {
	if phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserving &&
		!utilsres.SelfClusterAsInitiator(c.namespaceLister, trg.Spec.Initiator, trg.Annotations) {
		return true
	}
	return false
}

// needHandleExpiredTrg is used to check if trg is expired, if expired, patching the task resource group status phase to failed.
func (c *Controller) needHandleExpiredTrg(trg *kusciaapisv1alpha1.TaskResourceGroup) bool {
	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserved ||
		trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseFailed {
		return false
	}

	if !utilsres.SelfClusterAsInitiator(c.namespaceLister, trg.Spec.Initiator, trg.Annotations) &&
		trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseReserving {
		return false
	}

	now := metav1.Now()
	offset := time.Duration(trg.Spec.LifecycleSeconds) * time.Second
	expiredTime := trg.GetCreationTimestamp().Add(offset)
	nlog.Debugf("Task resource group expiredTime: %v, currentTime: %v", expiredTime, now)
	if !now.After(expiredTime) {
		return false
	}

	nlog.Infof("Task resource group %v is expired, patch the status phase %q to %q", trg.Name, trg.Status.Phase, kusciaapisv1alpha1.TaskResourceGroupPhaseFailed)
	trgCopy := trg.DeepCopy()
	trgCopy.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
	trgCopy.Status.LastTransitionTime = &now
	cond, _ := utilsres.GetTaskResourceGroupCondition(&trgCopy.Status, kusciaapisv1alpha1.TaskResourceGroupExpired)
	utilsres.SetTaskResourceGroupCondition(&now, cond, corev1.ConditionTrue, "Task resource group exceed it's lifecycle")

	err := utilsres.PatchTaskResourceGroupStatus(context.Background(), c.kusciaClient, trg, trgCopy)
	if err != nil {
		nlog.Errorf("Failed to handle expired task resource group %v and retry, %v", trgCopy.Name, err)
		c.trgQueue.AddRateLimited(trgCopy.Name)
	}
	return true
}

// needHandleReserveFailedTrg is used to check if trg should be handled when the status phase is reserve failed.
func (c *Controller) needHandleReserveFailedTrg(trg *kusciaapisv1alpha1.TaskResourceGroup) bool {
	if trg.Labels != nil && trg.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) {
		return false
	}

	now := metav1.Now()
	offset := time.Duration(trg.Spec.RetryIntervalSeconds) * time.Second
	retryTime := trg.Status.LastTransitionTime.Add(offset)
	nlog.Debugf("Task resource group retryTime: %v, currentTime: %v", retryTime, now)
	if now.After(retryTime) {
		return true
	}

	retryIntervalSeconds := trg.Spec.RetryIntervalSeconds
	if retryIntervalSeconds <= 0 {
		retryIntervalSeconds = defaultTaskResourceGroupRetryDurationSeconds
	}

	nlog.Infof("Put task resource group %q into reserve failed queue", trg.Name)
	c.trgReserveFailedQueue.AddAfter(trg.Name, time.Duration(retryIntervalSeconds)*time.Second)
	return false
}

// updateTaskResourceGroupStatus is used to update task resource group status.
func (c *Controller) updateTaskResourceGroupStatus(ctx context.Context, oldTrg, newTrg *kusciaapisv1alpha1.TaskResourceGroup) error {
	if reflect.DeepEqual(oldTrg.Status, newTrg.Status) {
		nlog.Debugf("Task resource group %v status is already updated, skip to update it", newTrg.Name)
		return nil
	}

	var err error
	newStatus := newTrg.Status.DeepCopy()
	trg := newTrg
	for i, trg := 0, trg; ; i++ {
		nlog.Infof("Start updating task resource group %q status phase %q to %q", newTrg.Name, oldTrg.Status.Phase, newTrg.Status.Phase)
		trg.Status = *newStatus
		_, err = c.kusciaClient.KusciaV1alpha1().TaskResourceGroups().UpdateStatus(ctx, trg, metav1.UpdateOptions{})
		if err == nil {
			nlog.Infof("Finish updating task resource group %q status phase %q to %q", newTrg.Name, oldTrg.Status.Phase, newTrg.Status.Phase)
			return nil
		}

		if k8serrors.IsConflict(err) {
			return nil
		}

		nlog.Warnf("Failed to update task resource group %q status, %v", newTrg.Name, err)
		if i >= statusUpdateRetries {
			break
		}

		if trg, err = c.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(ctx, trg.Name, metav1.GetOptions{}); err != nil {
			return err
		}

		if reflect.DeepEqual(trg.Status, *newStatus) {
			nlog.Infof("Task resource group %v status is already updated, skip to update it", trg.Name)
			return nil
		}
	}
	return err
}

// handleExpiredTrg is used to handle expired trg.
func (c *Controller) handleExpiredTrg() {
	item, shutdown := c.trgLifecycleQueue.Get()
	if shutdown {
		nlog.Info("Task resource group lifecycle queue is shutdown")
		return
	}

	nlog.Infof("Enqueue expired task resource group %v into trg queue", item)
	c.handleDelayingTrg(item)
	c.trgLifecycleQueue.Done(item)
}

// handleReserveFailedTrg is used to handle reserve failed trg.
func (c *Controller) handleReserveFailedTrg() {
	item, shutdown := c.trgReserveFailedQueue.Get()
	if shutdown {
		nlog.Info("Task resource group reserve failed queue is shutdown")
		return
	}

	nlog.Infof("Enqueue reserve failed task resource group %v into trg queue", item)
	c.handleDelayingTrg(item)
	c.trgReserveFailedQueue.Done(item)
}

// handleDelayingTrg is used to handle delaying task resource group.
func (c *Controller) handleDelayingTrg(item interface{}) {
	trgName, ok := item.(string)
	if !ok {
		nlog.Warnf("Task resource group lifecycle queue item key %v should be string type", item)
		return
	}

	c.trgQueue.Add(trgName)
}

// Name returns controller name.
func (c *Controller) Name() string {
	return controllerName
}
