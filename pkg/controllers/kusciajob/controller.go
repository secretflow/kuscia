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

package kusciajob

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/kusciajob/handler"
	"github.com/secretflow/kuscia/pkg/controllers/kusciajob/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	clientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	maxBackoffLimit     = 3
	controllerName      = "kuscia-job-controller"
	maxRetries          = 3
	statusUpdateRetries = 3
)

// Controller is the implementation for KusciaJob resources
type Controller struct {
	ctx    context.Context
	cancel context.CancelFunc
	// kusciaClient is a clientset for our own API group
	kusciaClient clientset.Interface

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
	// Handler Factory
	handlerFactory *handler.KusciaJobPhaseHandlerFactory

	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory

	// Lister and Synced of KusciaTask, KusciaJob
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
	kusciaTaskSynced cache.InformerSynced
	kusciaJobLister  kuscialistersv1alpha1.KusciaJobLister
	kusciaJobSynced  cache.InformerSynced

	namespaceLister listers.NamespaceLister
	namespaceSynced cache.InformerSynced
}

// NewController is used to new kuscia job controller.
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	eventRecorder := config.EventRecorder
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 5*time.Minute)

	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	controller := &Controller{
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		kusciaTaskLister:      kusciaTaskInformer.Lister(),
		kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
		kusciaJobLister:       kusciaJobInformer.Lister(),
		kusciaJobSynced:       kusciaJobInformer.Informer().HasSynced,
		namespaceLister:       namespaceInformer.Lister(),
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "kusciajob"),
		recorder:              eventRecorder,
	}
	controller.ctx, controller.cancel = context.WithCancel(ctx)

	controller.handlerFactory = handler.NewKusciaJobPhaseHandlerFactory(&handler.Dependencies{
		KusciaClient:     kusciaClient,
		Recorder:         eventRecorder,
		KusciaTaskLister: kusciaTaskInformer.Lister(),
		NamespaceLister:  namespaceInformer.Lister(),
	})

	// kuscia job event handler
	kusciaJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueKusciaJob,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueueKusciaJob(newObj)
		},
		DeleteFunc: controller.enqueueKusciaJob,
	})

	// kuscia task event handler
	kusciaTaskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleTaskObject,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.handleTaskObject(newObj)
		},
		DeleteFunc: controller.handleTaskObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	nlog.Infof("Starting %v", c.Name())
	c.kusciaInformerFactory.Start(c.ctx.Done())
	c.kubeInformerFactory.Start(c.ctx.Done())

	// Wait for the caches to be synced before starting workers
	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.kusciaTaskSynced, c.kusciaJobSynced, c.namespaceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// Launch workers to process KusciaTask resources
	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	for i := 0; i < workers; i++ {
		go c.runWorker()
	}
	nlog.Infof("Started workers, total %v", workers)

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

// enqueueKusciaJob takes a KusciaJob resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than KusciaTask.
func (c *Controller) enqueueKusciaJob(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		nlog.Errorf("Error building key of kusciaJob: %v", err)
	}
	c.workqueue.Add(key)
	nlog.Debugf("Enqueue kusciaJob %q", key)
}

// handleTaskObject enqueue the KusciaJob which the task belongs.
func (c *Controller) handleTaskObject(obj interface{}) {
	var object metav1.Object
	var ok bool
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

	nlog.Debugf("List-watch: Receive object: %v", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a KusciaJob, we should not do anything more with it.
		if ownerRef.Kind != handler.KusciaJobKind {
			nlog.Debugf("Task %v/%v not belong to this controller, ignore", object.GetNamespace(), object.GetName())
			return
		}
		kusciaJob, err := c.kusciaJobLister.Get(ownerRef.Name)
		if err != nil {
			nlog.Debugf("Ignoring orphaned object %q of kusciaJob %q", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueKusciaJob(kusciaJob)
		return
	}
}

// runWorker is a long-running function that will continually process queue items.
func (c *Controller) runWorker() {
	for queue.HandleQueueItem(c.ctx, controllerName, c.workqueue, c.syncHandler, maxRetries) {
		metrics.JobWorkerQueueSize.Set(float64(c.workqueue.Len()))
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the KusciaTask resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) (retErr error) {
	startTime := time.Now()

	// Convert the namespace/name string into a distinct namespace and name.
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Invalid resource key: %s", key)
		return nil
	}
	// Get the KusciaJob resource with this namespace/name.
	preJob, err := c.kusciaJobLister.Get(name)
	if err != nil {
		// The KusciaJob resource may no longer exist, in which case we stop processing.
		if k8serrors.IsNotFound(err) {
			metrics.ClearDeadMetrics(key)
			nlog.Infof("KusciaJob %q in work queue has been deleted", key)
			return nil
		}
		return err
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance.
	curJob := preJob.DeepCopy()
	// Set default for the new kusciaJob.
	kusciaJobDefault(curJob)

	// For kusciaJob that should not reconcile again, just return.
	if !handler.ShouldReconcile(curJob) {
		nlog.Infof("KusciaJob %q should not reconcile again, skipping", key)
		return nil
	}

	// For kusciaJob, we set default value to field.
	phase := curJob.Status.Phase
	if phase == "" {
		curJob.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	}

	// Internal state machine flow.
	needUpdate, err := c.handlerFactory.KusciaJobPhaseHandlerFor(phase).HandlePhase(curJob)
	if err != nil {
		metrics.JobSyncDurations.WithLabelValues(string(phase), metrics.Failed).Observe(time.Since(startTime).Seconds())
		if c.workqueue.NumRequeues(key) <= maxBackoffLimit {
			return fmt.Errorf("failed to handle condition for kusciaJob %q, detail-> %v", key, err)
		}

		c.failKusciaJob(curJob, fmt.Errorf("KusciaJob failed after %vx retry, last error: %v", maxBackoffLimit, err))
		needUpdate = true
	} else {
		metrics.JobSyncDurations.WithLabelValues(string(phase), metrics.Succeeded).Observe(time.Since(startTime).Seconds())
	}

	if !needUpdate {
		return nil
	}

	if err = utilsres.UpdateKusciaJobStatus(c.kusciaClient, preJob, curJob, statusUpdateRetries); err != nil {
		return err
	}

	nlog.Infof("Finished syncing KusciaJob %q (%v)", key, time.Since(startTime))
	return nil
}

// kusciaJobDefault will set kuscia job default field.
func kusciaJobDefault(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
	if kusciaJob.Status.Phase == "" {
		kusciaJob.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	}
	if kusciaJob.Spec.MaxParallelism == nil {
		maxParallelism := 1
		kusciaJob.Spec.MaxParallelism = &maxParallelism
	}
}

// failKusciaJob fill kuscia with error.
func (c *Controller) failKusciaJob(kusciaJob *kusciaapisv1alpha1.KusciaJob, err error) {
	now := metav1.Now()
	if kusciaJob.Status.StartTime == nil {
		kusciaJob.Status.StartTime = &now
	}
	kusciaJob.Status.Phase = kusciaapisv1alpha1.KusciaJobFailed
	kusciaJob.Status.Message = err.Error()
	kusciaJob.Status.CompletionTime = &now
	kusciaJob.Status.LastReconcileTime = &now
}

// Name returns the controller name.
func (c *Controller) Name() string {
	return controllerName
}
