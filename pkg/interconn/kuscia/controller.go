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

//nolint:dulp
package kuscia

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	iccommon "github.com/secretflow/kuscia/pkg/interconn/common"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	maxRetries    = 20
	defaultResync = 5 * time.Minute
)

const (
	controllerName             = "interconn-kuscia"
	interopConfigQueueName     = "interconn-kuscia-interopconfig-queue"
	deploymentQueueName        = "interconn-kuscia-deployment-queue"
	deploymentSummaryQueueName = "interconn-kuscia-deploymentsummary-queue"
	jobQueueName               = "interconn-kuscia-job-queue"
	jobSummaryQueueName        = "interconn-kuscia-jobsummary-queue"
	taskQueueName              = "interconn-kuscia-task-queue"
	taskSummaryQueueName       = "interconn-kuscia-tasksummary-queue"
	taskResourceQueueName      = "interconn-kuscia-taskresource-queue"
)

// Controller is the implementation for managing resources.
type Controller struct {
	mu                      sync.Mutex
	ctx                     context.Context
	cancel                  context.CancelFunc
	interopConfigInfos      map[string]*interopConfigInfo
	hostResourceManager     hostresources.ResourcesManager
	kusciaClient            kusciaclientset.Interface
	kusciaInformerFactory   kusciainformers.SharedInformerFactory
	interopConfigSynced     cache.InformerSynced
	interopConfigLister     kuscialistersv1alpha1.InteropConfigLister
	domainSynced            cache.InformerSynced
	domainLister            kuscialistersv1alpha1.DomainLister
	deploymentSynced        cache.InformerSynced
	deploymentLister        kuscialistersv1alpha1.KusciaDeploymentLister
	deploymentSummarySynced cache.InformerSynced
	deploymentSummaryLister kuscialistersv1alpha1.KusciaDeploymentSummaryLister
	jobSynced               cache.InformerSynced
	jobLister               kuscialistersv1alpha1.KusciaJobLister
	jobSummarySynced        cache.InformerSynced
	jobSummaryLister        kuscialistersv1alpha1.KusciaJobSummaryLister
	taskSynced              cache.InformerSynced
	taskLister              kuscialistersv1alpha1.KusciaTaskLister
	taskSummarySynced       cache.InformerSynced
	taskSummaryLister       kuscialistersv1alpha1.KusciaTaskSummaryLister
	taskResourceSynced      cache.InformerSynced
	taskResourceLister      kuscialistersv1alpha1.TaskResourceLister
	domainDataSynced        cache.InformerSynced
	domainDataLister        kuscialistersv1alpha1.DomainDataLister
	domainDataGrantSynced   cache.InformerSynced
	domainDataGrantLister   kuscialistersv1alpha1.DomainDataGrantLister
	interopConfigQueue      workqueue.RateLimitingInterface
	deploymentQueue         workqueue.RateLimitingInterface
	deploymentSummaryQueue  workqueue.RateLimitingInterface
	jobQueue                workqueue.RateLimitingInterface
	jobSummaryQueue         workqueue.RateLimitingInterface
	taskQueue               workqueue.RateLimitingInterface
	taskSummaryQueue        workqueue.RateLimitingInterface
	taskResourceQueue       workqueue.RateLimitingInterface
}

type interopConfigInfo struct {
	host    string
	members []string
}

// NewController returns a controller instance.
func NewController(ctx context.Context, kubeClient kubernetes.Interface, kusciaClient kusciaclientset.Interface, eventRecorder record.EventRecorder) iccommon.IController {
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, defaultResync)
	interopConfigInformer := kusciaInformerFactory.Kuscia().V1alpha1().InteropConfigs()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	deploymentInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	deploymentSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeploymentSummaries()
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	jobSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobSummaries()
	taskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	taskResourceInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	domainDataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	domainDataGrantInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()
	hMgrOpt := &hostresources.Options{
		MemberKusciaClient:          kusciaClient,
		MemberDomainLister:          domainInformer.Lister(),
		MemberDeploymentLister:      deploymentInformer.Lister(),
		MemberJobLister:             jobInformer.Lister(),
		MemberTaskLister:            taskInformer.Lister(),
		MemberTaskResourceLister:    taskResourceInformer.Lister(),
		MemberDomainDataGrantLister: domainDataGrantInformer.Lister(),
		MemberDomainDataLister:      domainDataInformer.Lister(),
	}

	controller := &Controller{
		interopConfigInfos:      make(map[string]*interopConfigInfo),
		hostResourceManager:     hostresources.NewHostResourcesManager(hMgrOpt),
		kusciaClient:            kusciaClient,
		kusciaInformerFactory:   kusciaInformerFactory,
		interopConfigSynced:     interopConfigInformer.Informer().HasSynced,
		interopConfigLister:     interopConfigInformer.Lister(),
		domainSynced:            domainInformer.Informer().HasSynced,
		domainLister:            domainInformer.Lister(),
		deploymentSynced:        deploymentInformer.Informer().HasSynced,
		deploymentLister:        deploymentInformer.Lister(),
		deploymentSummarySynced: deploymentSummaryInformer.Informer().HasSynced,
		deploymentSummaryLister: deploymentSummaryInformer.Lister(),
		jobSynced:               jobInformer.Informer().HasSynced,
		jobLister:               jobInformer.Lister(),
		jobSummarySynced:        jobSummaryInformer.Informer().HasSynced,
		jobSummaryLister:        jobSummaryInformer.Lister(),
		taskSynced:              taskInformer.Informer().HasSynced,
		taskLister:              taskInformer.Lister(),
		taskSummarySynced:       taskSummaryInformer.Informer().HasSynced,
		taskSummaryLister:       taskSummaryInformer.Lister(),
		taskResourceSynced:      taskResourceInformer.Informer().HasSynced,
		taskResourceLister:      taskResourceInformer.Lister(),
		domainDataSynced:        domainDataInformer.Informer().HasSynced,
		domainDataLister:        domainDataInformer.Lister(),
		domainDataGrantSynced:   domainDataGrantInformer.Informer().HasSynced,
		domainDataGrantLister:   domainDataGrantInformer.Lister(),
		interopConfigQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), interopConfigQueueName),
		deploymentQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), deploymentQueueName),
		deploymentSummaryQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), deploymentSummaryQueueName),
		jobQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), jobQueueName),
		jobSummaryQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), jobSummaryQueueName),
		taskQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskQueueName),
		taskSummaryQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskSummaryQueueName),
		taskResourceQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskResourceQueueName),
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)
	interopConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAddedorDeletedInteropConfig,
		UpdateFunc: controller.handleUpdatedInteropConfig,
		DeleteFunc: controller.handleAddedorDeletedInteropConfig,
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedDeployment,
			UpdateFunc: controller.handleUpdatedDeployment,
			DeleteFunc: controller.handleDeletedDeployment,
		},
	})

	deploymentSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedDeploymentSummary,
			UpdateFunc: controller.handleUpdatedDeploymentSummary,
		},
	})

	jobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedJob,
			UpdateFunc: controller.handleUpdatedJob,
			DeleteFunc: controller.handleDeletedJob,
		},
	})

	jobSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedJobSummary,
			UpdateFunc: controller.handleUpdatedJobSummary,
		},
	})

	taskInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedTask,
			UpdateFunc: controller.handleUpdatedTask,
			DeleteFunc: controller.handleDeletedTask,
		},
	})

	taskSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedTaskSummary,
			UpdateFunc: controller.handleUpdatedTaskSummary,
		},
	})

	taskResourceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedTaskResource,
			UpdateFunc: controller.handleUpdatedTaskResource,
		},
	})

	return controller
}

// resourceFilter is used to filter specific resources.
func (c *Controller) resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch o := obj.(type) {
		case *kusciaapisv1alpha1.KusciaDeployment:
			return filterDeployment(o)
		case *kusciaapisv1alpha1.KusciaJob:
			return filterJob(o)
		case *kusciaapisv1alpha1.KusciaTask:
			return filterTask(o)
		case *kusciaapisv1alpha1.TaskResource:
			return filterTaskResource(o)
		case *kusciaapisv1alpha1.KusciaJobSummary:
			return filterResourceSummary(o)
		case *kusciaapisv1alpha1.KusciaTaskSummary:
			return filterResourceSummary(o)
		case *kusciaapisv1alpha1.KusciaDeploymentSummary:
			return filterResourceSummary(o)
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

// filterJob is used to filter job that meet the requirements.
func filterJob(obj metav1.Object) bool {
	if obj.GetNamespace() != common.KusciaCrossDomain {
		return false
	}
	return matchAnnotations(obj.GetAnnotations())
}

// filterDeployment is used to filter deployment that meet the requirements.
func filterDeployment(obj metav1.Object) bool {
	return matchAnnotations(obj.GetAnnotations())
}

// filterTask is used to filter kuscia task that meet the requirements.
func filterTask(obj metav1.Object) bool {
	if common.KusciaCrossDomain != obj.GetNamespace() {
		return false
	}

	annotations := obj.GetAnnotations()
	if annotations == nil ||
		annotations[common.TaskAliasAnnotationKey] == "" ||
		annotations[common.JobIDAnnotationKey] == "" {
		return false
	}

	return matchAnnotations(obj.GetAnnotations())
}

func matchAnnotations(annotations map[string]string) bool {
	if annotations == nil ||
		annotations[common.InitiatorAnnotationKey] == "" ||
		annotations[common.InterConnKusciaPartyAnnotationKey] == "" ||
		annotations[common.InterConnSelfPartyAnnotationKey] == "" {
		return false
	}

	return true
}

// filterTaskResource is used to filter kuscia taskresource that meet the requirements.
func filterTaskResource(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil ||
		labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnKuscia) {
		return false
	}

	annotations := obj.GetAnnotations()
	if annotations == nil ||
		annotations[common.InitiatorAnnotationKey] == "" ||
		annotations[common.TaskIDAnnotationKey] == "" {
		return false
	}
	return true
}

// filterResourceSummary is used to filter deploymentSummary, jobSummary and taskSummary that meet the requirements.
func filterResourceSummary(obj metav1.Object) bool {
	if obj.GetDeletionTimestamp() != nil {
		return false
	}

	return matchResourceSummaryAnnotations(obj.GetAnnotations())
}

func matchResourceSummaryAnnotations(annotations map[string]string) bool {
	if annotations == nil ||
		annotations[common.InitiatorAnnotationKey] == "" ||
		annotations[common.InterConnKusciaPartyAnnotationKey] == "" {
		return false
	}
	return true
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer func() {
		c.interopConfigQueue.ShutDown()
		c.deploymentQueue.ShutDown()
		c.deploymentSummaryQueue.ShutDown()
		c.jobQueue.ShutDown()
		c.jobSummaryQueue.ShutDown()
		c.taskQueue.ShutDown()
		c.taskSummaryQueue.ShutDown()
		c.taskResourceQueue.ShutDown()
	}()

	c.hostResourceManager.SetWorkers(workers)
	nlog.Infof("Starting %v", c.Name())
	c.kusciaInformerFactory.Start(c.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if !cache.WaitForCacheSync(c.ctx.Done(),
		c.domainSynced,
		c.interopConfigSynced,
		c.deploymentSynced, c.deploymentSummarySynced,
		c.jobSynced, c.jobSummarySynced,
		c.taskSynced, c.taskSummarySynced, c.taskResourceSynced,
		c.domainDataSynced, c.domainDataGrantSynced) {
		return fmt.Errorf("failed to wait for cache sync for %v", c.Name())
	}
	c.registerInteropConfigs()
	for {
		if c.hostResourceManager.HasSynced() {
			nlog.Info("Host resource manager finish syncing resources")
			break
		}
		nlog.Info("Waiting for host resource manager to sync resources...")
		time.Sleep(2 * time.Second)
	}

	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(c.ctx, c.runInteropConfigWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runDeploymentWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runDeploymentSummaryWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runJobWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runJobSummaryWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskSummaryWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runTaskResourceWorker, time.Second)
	}

	<-c.ctx.Done()
	return nil
}

// Stop is used to stop the controller.
func (c *Controller) Stop() {
	c.hostResourceManager.Stop()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// Name returns the controller name.
func (c *Controller) Name() string {
	return controllerName
}

const crdInteropConfigsName = "interopconfigs.kuscia.secretflow"
