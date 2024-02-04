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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	gochache "github.com/patrickmn/go-cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/client"
	bfiacommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	iccommon "github.com/secretflow/kuscia/pkg/interconn/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	kusciaJobQueueName            = "interconn-bfia-kusciajob-queue"
	kusciaTaskQueueName           = "interconn-bfia-kusciatask-queue"
	taskResourceQueueName         = "interconn-bfia-taskresource-queue"
	kusciaTaskStatusSyncQueueName = "interconn-bfia-kusciatask-status-sync-queue"
	kusciaJobStatusSyncQueueName  = "interconn-bfia-kusciajob-status-sync-queue"
)

const (
	maxRetries                             = 5
	inflightRequestCacheExpiration         = 600 * time.Second
	finishedInflightRequestCacheExpiration = 15 * time.Second
	taskStatusSyncInterval                 = 5 * time.Second
	jobStatusSyncInterval                  = 15 * time.Second
	statusUpdateRetries                    = 5
)

const (
	interconnSchedulerSvc = "interconn-scheduler.%s.svc"
)

type requestType string

const (
	reqTypeCreateJob               requestType = "create_job"
	reqTypeStopJob                 requestType = "stop_job"
	reqTypeStartJob                requestType = "start_job"
	reqTypeQueryJobStatus          requestType = "query_job_status"
	reqTypeStartTask               requestType = "start_task"
	reqTypeStopTask                requestType = "stop_task"
	reqTypeQueryTaskStatusWithPoll requestType = "query_task_with_poll"
)

type resourceType string

const (
	resourceTypeKusciaJob    resourceType = "kusciaJob"
	resourceTypeTaskResource resourceType = "taskResource"
	resourceTypeKusciaTask   resourceType = "kusciaTask"
)

// Controller is the implementation for managing resources.
type Controller struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kubeInformerFactory   informers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	nsLister              listers.NamespaceLister
	nsSynced              cache.InformerSynced
	kjLister              kuscialistersv1alpha1.KusciaJobLister
	kjSynced              cache.InformerSynced
	ktLister              kuscialistersv1alpha1.KusciaTaskLister
	ktSynced              cache.InformerSynced
	appImageLister        kuscialistersv1alpha1.AppImageLister
	appImageSynced        cache.InformerSynced
	trLister              kuscialistersv1alpha1.TaskResourceLister
	trSynced              cache.InformerSynced
	ktStatusSyncQueue     workqueue.DelayingInterface
	kjStatusSyncQueue     workqueue.DelayingInterface
	trQueue               workqueue.RateLimitingInterface
	kjQueue               workqueue.RateLimitingInterface
	ktQueue               workqueue.RateLimitingInterface
	recorder              record.EventRecorder

	bfiaClient           *client.Client
	inflightRequestCache *gochache.Cache
}

// NewController returns a controller instance.
func NewController(ctx context.Context, kubeClient kubernetes.Interface, kusciaClient kusciaclientset.Interface, eventRecorder record.EventRecorder) iccommon.IController {
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 5*time.Minute)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()

	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	controller := &Controller{
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		nsLister:              nsInformer.Lister(),
		nsSynced:              nsInformer.Informer().HasSynced,
		kjLister:              kjInformer.Lister(),
		kjSynced:              kjInformer.Informer().HasSynced,
		ktLister:              ktInformer.Lister(),
		ktSynced:              ktInformer.Informer().HasSynced,
		appImageLister:        appImageInformer.Lister(),
		appImageSynced:        appImageInformer.Informer().HasSynced,
		trLister:              trInformer.Lister(),
		trSynced:              trInformer.Informer().HasSynced,
		kjQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), kusciaJobQueueName),
		ktQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), kusciaTaskQueueName),
		trQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskResourceQueueName),
		ktStatusSyncQueue:     workqueue.NewNamedDelayingQueue(kusciaTaskStatusSyncQueueName),
		kjStatusSyncQueue:     workqueue.NewNamedDelayingQueue(kusciaJobStatusSyncQueueName),
		recorder:              eventRecorder,
		bfiaClient:            client.New(),
		inflightRequestCache:  gochache.New(inflightRequestCacheExpiration, inflightRequestCacheExpiration),
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)
	kjInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedOrDeletedKusciaJob,
			UpdateFunc: controller.handleUpdatedKusciaJob,
			DeleteFunc: controller.handleAddedOrDeletedKusciaJob,
		},
	})

	trInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedOrDeletedTaskResource,
			UpdateFunc: controller.handleUpdatedTaskResource,
			DeleteFunc: controller.handleAddedOrDeletedTaskResource,
		},
	})

	ktInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedOrDeletedKusciaTask,
			DeleteFunc: controller.handleAddedOrDeletedKusciaTask,
		},
	})

	return controller
}

// resourceFilter is used to filter resource.
func (c *Controller) resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch t := obj.(type) {
		case *kusciaapisv1alpha1.KusciaJob:
			return c.matchLabels(t)
		case *kusciaapisv1alpha1.TaskResource:
			return c.taskResourceMatchLabels(t)
		case *kusciaapisv1alpha1.KusciaTask:
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

// jobMatchLabels is used to filter resources.
func (c *Controller) matchLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	if labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnBFIA) {
		return false
	}

	return true
}

// taskResourceMatchLabels is used to filter task resource.
func (c *Controller) taskResourceMatchLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	if labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnBFIA) {
		return false
	}

	ns := obj.GetNamespace()
	if !utilsres.IsOuterBFIAInterConnDomain(c.nsLister, ns) {
		return false
	}

	tr, ok := obj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		return false
	}

	if tr.Status.Phase != kusciaapisv1alpha1.TaskResourcePhaseReserving {
		return false
	}

	if !utilsres.SelfClusterAsInitiator(c.nsLister, tr.Spec.Initiator, nil) {
		return false
	}

	return true
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the work queue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer func() {
		c.kjQueue.ShutDown()
		c.ktStatusSyncQueue.ShutDown()
		c.kjStatusSyncQueue.ShutDown()
		c.trQueue.ShutDown()
	}()

	nlog.Infof("Starting %v", c.Name())
	c.kubeInformerFactory.Start(c.ctx.Done())
	c.kusciaInformerFactory.Start(c.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.nsSynced, c.kjSynced, c.ktSynced, c.appImageSynced, c.trSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(c.ctx, c.runJobWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskResourceWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runJobStatusSyncWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskStatusSyncWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskWorker, time.Second)
	}

	<-c.ctx.Done()
	return nil
}

// Stop used to stop the controller.
func (c *Controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// Name returns controller name.
func (c *Controller) Name() string {
	return bfiacommon.ServerName
}

// updatePartyTaskStatus updates party task  status of kuscia task.
func (c *Controller) updatePartyTaskStatus(task *kusciaapisv1alpha1.KusciaTask) (err error) {
	partyTaskStatus := make([]kusciaapisv1alpha1.PartyTaskStatus, len(task.Status.PartyTaskStatus))
	copy(partyTaskStatus, task.Status.PartyTaskStatus)
	taskName := task.Name
	for i, task := 0, task; ; i++ {
		nlog.Infof("Start updating kuscia task %q party status %+v", taskName, task.Status.PartyTaskStatus)
		if ret, err := c.kusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).UpdateStatus(context.Background(), task, metav1.UpdateOptions{}); err == nil {
			nlog.Infof("Finish updating kuscia task %q party status %+v", taskName, ret.Status.PartyTaskStatus)
			return nil
		}

		nlog.Warnf("Failed to update kuscia task %q party status, %v", taskName, err)
		if i >= statusUpdateRetries {
			break
		}

		task, err = c.kusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).Get(context.Background(), taskName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get the newest kuscia task %q, %v", taskName, err)
		}

		if reflect.DeepEqual(partyTaskStatus, task.Status.PartyTaskStatus) {
			return nil
		}

		utilsres.MergeKusciaTaskPartyTaskStatus(task, partyTaskStatus)
	}
	return err
}

func (c *Controller) cleanCacheData(reqType requestType, resType resourceType, resourceName string) {
	c.inflightRequestCache.Delete(getCacheKeyName(reqType, resType, resourceName))
}

func getCacheKeyName(reqType requestType, resourceType resourceType, name string) string {
	return string(resourceType) + "/" + string(reqType) + "/" + name
}

func buildHostFor(domainID string) string {
	return fmt.Sprintf(interconnSchedulerSvc, domainID)
}

func setPartyTaskStatus(status *kusciaapisv1alpha1.KusciaTaskStatus, domainID, role, message string, phase kusciaapisv1alpha1.KusciaTaskPhase) bool {
	for idx, s := range status.PartyTaskStatus {
		if s.DomainID == domainID && s.Role == role {
			if s.Phase == phase {
				return false
			}

			if s.Phase == kusciaapisv1alpha1.TaskFailed || s.Phase == kusciaapisv1alpha1.TaskSucceeded {
				return false
			}

			status.PartyTaskStatus[idx].Phase = phase
			status.PartyTaskStatus[idx].Message = message
			return true
		}
	}

	status.PartyTaskStatus = append(status.PartyTaskStatus, kusciaapisv1alpha1.PartyTaskStatus{
		DomainID: domainID,
		Role:     role,
		Phase:    phase,
		Message:  message,
	})
	return true
}
