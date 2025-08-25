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

package hostresources

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	maxRetries    = 20
	defaultResync = 5 * time.Minute
)

// ResourcesAccessor is an interface to get host resources controller related information.
type ResourcesAccessor interface {
	HasSynced() bool
	HostKusciaClient() kusciaclientset.Interface
	HostJobLister() kuscialistersv1alpha1.KusciaJobLister
	HostJobSummaryLister() kuscialistersv1alpha1.KusciaJobSummaryLister
	HostTaskSummaryLister() kuscialistersv1alpha1.KusciaTaskSummaryLister
	HostDeploymentLister() kuscialistersv1alpha1.KusciaDeploymentLister
	HostDeploymentSummaryLister() kuscialistersv1alpha1.KusciaDeploymentSummaryLister
	HostDomainDataLister() kuscialistersv1alpha1.DomainDataLister
	HostDomainDataGrantLister() kuscialistersv1alpha1.DomainDataGrantLister
	EnqueueDeployment(string)
}

// hostResourcesController is used to manage host resources.
type hostResourcesController struct {
	host                        string
	member                      string
	hasSynced                   bool
	stopCh                      chan struct{}
	hostKusciaClient            kusciaclientset.Interface
	hostKusciaInformerFactory   kusciainformers.SharedInformerFactory
	hostDeploymentSynced        cache.InformerSynced
	hostDeploymentLister        kuscialistersv1alpha1.KusciaDeploymentLister
	hostDeploymentSummarySynced cache.InformerSynced
	hostDeploymentSummaryLister kuscialistersv1alpha1.KusciaDeploymentSummaryLister
	hostJobSynced               cache.InformerSynced
	hostJobLister               kuscialistersv1alpha1.KusciaJobLister
	hostJobSummarySynced        cache.InformerSynced
	hostJobSummaryLister        kuscialistersv1alpha1.KusciaJobSummaryLister
	hostTaskSummarySynced       cache.InformerSynced
	hostTaskSummaryLister       kuscialistersv1alpha1.KusciaTaskSummaryLister
	hostDomainDataSynced        cache.InformerSynced
	hostDomainDataLister        kuscialistersv1alpha1.DomainDataLister
	hostDomainDataGrantSynced   cache.InformerSynced
	hostDomainDataGrantLister   kuscialistersv1alpha1.DomainDataGrantLister
	memberKusciaClient          kusciaclientset.Interface
	memberDomainLister          kuscialistersv1alpha1.DomainLister
	memberDeploymentLister      kuscialistersv1alpha1.KusciaDeploymentLister
	memberJobLister             kuscialistersv1alpha1.KusciaJobLister
	memberTaskLister            kuscialistersv1alpha1.KusciaTaskLister
	memberTaskResourceLister    kuscialistersv1alpha1.TaskResourceLister
	memberDomainDataLister      kuscialistersv1alpha1.DomainDataLister
	memberDomainDataGrantLister kuscialistersv1alpha1.DomainDataGrantLister
	deploymentQueueName         string
	deploymentQueue             workqueue.RateLimitingInterface
	deploymentSummaryQueueName  string
	deploymentSummaryQueue      workqueue.RateLimitingInterface
	jobQueueName                string
	jobQueue                    workqueue.RateLimitingInterface
	jobSummaryQueueName         string
	jobSummaryQueue             workqueue.RateLimitingInterface
	taskSummaryQueueName        string
	taskSummaryQueue            workqueue.RateLimitingInterface
	domainDataGrantQueueName    string
	domainDataGrantQueue        workqueue.RateLimitingInterface
	domainDataQueueName         string
	domainDataQueue             workqueue.RateLimitingInterface
}

// hostResourcesControllerOptions defines some options for host resources controller.
type hostResourcesControllerOptions struct {
	host                        string
	member                      string
	memberKusciaClient          kusciaclientset.Interface
	memberDomainLister          kuscialistersv1alpha1.DomainLister
	memberDeploymentLister      kuscialistersv1alpha1.KusciaDeploymentLister
	memberJobLister             kuscialistersv1alpha1.KusciaJobLister
	memberTaskLister            kuscialistersv1alpha1.KusciaTaskLister
	memberTaskResourceLister    kuscialistersv1alpha1.TaskResourceLister
	memberDomainDataLister      kuscialistersv1alpha1.DomainDataLister
	memberDomainDataGrantLister kuscialistersv1alpha1.DomainDataGrantLister
}

// newHostResourcesController returns a host resources controller instance.
func newHostResourcesController(opts *hostResourcesControllerOptions) (*hostResourcesController, error) {
	host := opts.host
	member := opts.member

	clients, err := GetHostClient("", getAPIServerURLForHostCluster(host))
	if err != nil {
		nlog.Errorf("New host resources controller failed, %v", err.Error())
		return nil, err
	}

	hKusciaInformerFactory := kusciainformers.NewSharedInformerFactoryWithOptions(clients.KusciaClient, defaultResync, kusciainformers.WithNamespace(member))
	hDeploymentInformer := hKusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	hDeploymentSummaryInformer := hKusciaInformerFactory.Kuscia().V1alpha1().KusciaDeploymentSummaries()
	hJobInformer := hKusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	hJobSummaryInformer := hKusciaInformerFactory.Kuscia().V1alpha1().KusciaJobSummaries()
	hTaskSummaryInformer := hKusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	hDomainDataInformer := hKusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	hDomainDataGrantInformer := hKusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()

	deploymentQueueName := fmt.Sprintf("host-%s-member-%s-kd-queue", host, member)
	deploymentSummaryQueueName := fmt.Sprintf("host-%v-member-%v-kds-queue", host, member)
	jobQueueName := fmt.Sprintf("host-%s-member-%s-job-queue", host, member)
	jobSummaryQueueName := fmt.Sprintf("host-%v-member-%v-jobsummary-queue", host, member)
	taskSummaryQueueName := fmt.Sprintf("host-%v-member-%v-tasksummary-queue", host, member)
	domainDataGrantQueueName := fmt.Sprintf("host-%v-member-%v-domaindatagrant-queue", host, member)
	domainDataQueueName := fmt.Sprintf("host-%v-member-%v-domaindata-queue", host, member)

	stopCh := make(chan struct{})
	hrc := &hostResourcesController{
		stopCh:                      stopCh,
		host:                        host,
		member:                      member,
		hostKusciaClient:            clients.KusciaClient,
		hostKusciaInformerFactory:   hKusciaInformerFactory,
		hostDeploymentSynced:        hDeploymentInformer.Informer().HasSynced,
		hostDeploymentLister:        hDeploymentInformer.Lister(),
		hostDeploymentSummarySynced: hDeploymentSummaryInformer.Informer().HasSynced,
		hostDeploymentSummaryLister: hDeploymentSummaryInformer.Lister(),
		hostJobSynced:               hJobInformer.Informer().HasSynced,
		hostJobLister:               hJobInformer.Lister(),
		hostJobSummarySynced:        hJobSummaryInformer.Informer().HasSynced,
		hostJobSummaryLister:        hJobSummaryInformer.Lister(),
		hostTaskSummarySynced:       hTaskSummaryInformer.Informer().HasSynced,
		hostTaskSummaryLister:       hTaskSummaryInformer.Lister(),
		hostDomainDataGrantSynced:   hDomainDataGrantInformer.Informer().HasSynced,
		hostDomainDataGrantLister:   hDomainDataGrantInformer.Lister(),
		hostDomainDataSynced:        hDomainDataInformer.Informer().HasSynced,
		hostDomainDataLister:        hDomainDataInformer.Lister(),
		memberKusciaClient:          opts.memberKusciaClient,
		memberDomainLister:          opts.memberDomainLister,
		memberDeploymentLister:      opts.memberDeploymentLister,
		memberJobLister:             opts.memberJobLister,
		memberTaskLister:            opts.memberTaskLister,
		memberTaskResourceLister:    opts.memberTaskResourceLister,
		memberDomainDataLister:      opts.memberDomainDataLister,
		memberDomainDataGrantLister: opts.memberDomainDataGrantLister,
		deploymentQueueName:         deploymentQueueName,
		deploymentQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), deploymentQueueName),
		deploymentSummaryQueueName:  deploymentSummaryQueueName,
		deploymentSummaryQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), deploymentSummaryQueueName),
		jobQueueName:                jobQueueName,
		jobQueue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), jobQueueName),
		jobSummaryQueueName:         jobSummaryQueueName,
		jobSummaryQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), jobSummaryQueueName),
		taskSummaryQueueName:        taskSummaryQueueName,
		taskSummaryQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskSummaryQueueName),
		domainDataQueueName:         domainDataQueueName,
		domainDataQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), domainDataQueueName),
		domainDataGrantQueueName:    domainDataGrantQueueName,
		domainDataGrantQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), domainDataGrantQueueName),
	}

	_, _ = hDeploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedDeployment,
			UpdateFunc: hrc.handleUpdatedDeployment,
			DeleteFunc: hrc.handleDeletedDeployment,
		},
	})

	_, _ = hDeploymentSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedorDeletedDeploymentSummary,
			UpdateFunc: hrc.handleUpdatedDeploymentSummary,
			DeleteFunc: hrc.handleAddedorDeletedDeploymentSummary,
		},
	})

	_, _ = hJobInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedJob,
			UpdateFunc: hrc.handleUpdatedJob,
			DeleteFunc: hrc.handleDeletedJob,
		},
	})

	_, _ = hJobSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedorDeletedJobSummary,
			UpdateFunc: hrc.handleUpdatedJobSummary,
			DeleteFunc: hrc.handleAddedorDeletedJobSummary,
		},
	})

	_, _ = hTaskSummaryInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedorDeletedTaskSummary,
			UpdateFunc: hrc.handleUpdatedTaskSummary,
			DeleteFunc: hrc.handleAddedorDeletedTaskSummary,
		},
	})

	_, _ = hDomainDataGrantInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedDomainDataGrant,
			UpdateFunc: hrc.handleUpdatedDomainDataGrant,
			DeleteFunc: hrc.handleDeletedDomainDataGrant,
		},
	})

	_, _ = hDomainDataInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedDomainData,
			UpdateFunc: hrc.handleUpdatedDomainData,
			DeleteFunc: hrc.handleDeletedDomainData,
		},
	})
	return hrc, nil
}

// resourceFilter is used to filter resource.
func (c *hostResourcesController) resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch o := obj.(type) {
		case *kusciaapisv1alpha1.KusciaDeployment:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.KusciaDeploymentSummary:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.KusciaJob:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.KusciaJobSummary:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.KusciaTaskSummary:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.DomainDataGrant:
			return c.filterResources(o)
		case *kusciaapisv1alpha1.DomainData:
			return c.filterResources(o)
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

// filterResources is used to filter resources that meets the requirements.
func (c *hostResourcesController) filterResources(obj metav1.Object) bool {
	return c.matchAnnotations(obj.GetAnnotations())
}

// matchAnnotations is used to match obj annotations.
func (c *hostResourcesController) matchAnnotations(annotations map[string]string) bool {
	if annotations == nil ||
		annotations[common.InitiatorAnnotationKey] == "" ||
		annotations[common.InterConnKusciaPartyAnnotationKey] == "" {
		return false
	}
	return true
}

// run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *hostResourcesController) run(workers int) error {
	nlog.Infof("Start host resource controller for %v/%v", c.host, c.member)
	c.hostKusciaInformerFactory.Start(c.stopCh)

	nlog.Infof("Start waiting for cache to sync for %v/%v resource controller", c.host, c.member)
	if ok := cache.WaitForCacheSync(c.stopCh,
		c.hostDeploymentSynced, c.hostDeploymentSummarySynced,
		c.hostJobSynced, c.hostJobSummarySynced,
		c.hostTaskSummarySynced,
		c.hostDomainDataSynced, c.hostDomainDataGrantSynced); !ok {
		return fmt.Errorf("failed to wait for %v/%v cache to sync for resource controller", c.host, c.member)
	}
	nlog.Infof("Finish waiting for cache to sync for %v/%v resource controller", c.host, c.member)

	c.hasSynced = true
	// init first list and process domaindata domaindatagrant
	c.initKddQueue()
	c.initDdgQueue()

	nlog.Infof("Run workers for %v/%v resource controller", c.host, c.member)
	for i := 0; i < workers; i++ {
		go wait.Until(c.runDeploymentWorker, time.Second, c.stopCh)
		go wait.Until(c.runDeploymentSummaryWorker, time.Second, c.stopCh)
		go wait.Until(c.runJobWorker, time.Second, c.stopCh)
		go wait.Until(c.runJobSummaryWorker, time.Second, c.stopCh)
		go wait.Until(c.runTaskSummaryWorker, time.Second, c.stopCh)
		go wait.Until(c.runDomainDataWorker, time.Second, c.stopCh)
		go wait.Until(c.runDomainDataGrantWorker, time.Second, c.stopCh)
	}
	return nil
}

func (c *hostResourcesController) initKddQueue() {
	// list all domaindata granted from c.host, and put them into queue
	r, err := labels.NewRequirement(common.LabelDomainDataVendor, selection.Equals, []string{common.DomainDataVendorGrant})
	if err != nil {
		nlog.Errorf("Create label selector failed: %s, skip try to init kdd update.", err.Error())
	}
	if c.memberDomainDataLister == nil {
		nlog.Errorf("memberDomainDataLister is nil, skip try to init kdd update.")
		return
	}
	kdds, err := c.memberDomainDataLister.DomainDatas(c.member).List(labels.NewSelector().Add(*r))
	if err != nil {
		nlog.Errorf("List domaindata failed: %s, skip try to init kdd update.", err.Error())
	}
	for _, kdd := range kdds {
		if kdd.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
			continue
		}
		fakeNewObj := kdd.DeepCopy()
		fakeNewObj.ResourceVersion = "0"
		c.handleUpdatedDomainData(kdd, fakeNewObj)
	}
}

func (c *hostResourcesController) initDdgQueue() {
	// list all domaindatagrant granted from c.host, and put them into queue
	if c.memberDomainDataGrantLister == nil {
		nlog.Errorf("memberDomainDataGrantLister is nil, skip try to init ddg update.")
		return
	}
	ddgs, err := c.memberDomainDataGrantLister.DomainDataGrants(c.member).List(labels.Everything())
	if err != nil {
		nlog.Errorf("List domaindatagrant failed: %s, skip try to init ddg update.", err.Error())
	}
	for _, ddg := range ddgs {
		if ddg.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
			continue
		}
		fakeNewObj := ddg.DeepCopy()
		fakeNewObj.ResourceVersion = "0"
		c.handleUpdatedDomainDataGrant(ddg, fakeNewObj)
	}
}

// stop is used to stop the host resource controller.
func (c *hostResourcesController) stop() {
	c.deploymentQueue.ShutDown()
	c.deploymentSummaryQueue.ShutDown()
	c.jobQueue.ShutDown()
	c.jobSummaryQueue.ShutDown()
	c.taskSummaryQueue.ShutDown()
	c.domainDataQueue.ShutDown()
	c.domainDataGrantQueue.ShutDown()
	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}
	nlog.Infof("%v/%v resource controller stopped", c.host, c.member)
}

func (c *hostResourcesController) HasSynced() bool {
	return c.hasSynced
}

// HostKusciaClient returns KusciaClient of host cluster.
func (c *hostResourcesController) HostKusciaClient() kusciaclientset.Interface {
	return c.hostKusciaClient
}

// HostJobLister returns jobLister of host cluster.
func (c *hostResourcesController) HostJobLister() kuscialistersv1alpha1.KusciaJobLister {
	return c.hostJobLister
}

// HostJobSummaryLister returns jobSummaryLister of host cluster.
func (c *hostResourcesController) HostJobSummaryLister() kuscialistersv1alpha1.KusciaJobSummaryLister {
	return c.hostJobSummaryLister
}

// HostTaskSummaryLister returns taskSummaryLister of host cluster.
func (c *hostResourcesController) HostTaskSummaryLister() kuscialistersv1alpha1.KusciaTaskSummaryLister {
	return c.hostTaskSummaryLister
}

// HostDeploymentLister returns deploymentLister of host cluster.
func (c *hostResourcesController) HostDeploymentLister() kuscialistersv1alpha1.KusciaDeploymentLister {
	return c.hostDeploymentLister
}

// HostDeploymentSummaryLister returns deploymentSummaryLister of host cluster.
func (c *hostResourcesController) HostDeploymentSummaryLister() kuscialistersv1alpha1.KusciaDeploymentSummaryLister {
	return c.hostDeploymentSummaryLister
}

// HostDomainDataLister returns domainDataLister of host cluster.
func (c *hostResourcesController) HostDomainDataLister() kuscialistersv1alpha1.DomainDataLister {
	return c.hostDomainDataLister
}

// HostDomainDataGrantLister returns domainDataGrantLister of host cluster.
func (c *hostResourcesController) HostDomainDataGrantLister() kuscialistersv1alpha1.DomainDataGrantLister {
	return c.hostDomainDataGrantLister
}

// EnqueueDeployment is used to put deployment key into host deployments queue.
func (c *hostResourcesController) EnqueueDeployment(key string) {
	c.deploymentQueue.Add(key)
}

// GetHostClient returns kubeClient and kusciaClient of host cluster.
var GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
	return kubeconfig.CreateClientSetsFromToken(token, masterURL)
}

// getAPIServerURLForHostCluster returns api server url for host cluster.
func getAPIServerURLForHostCluster(host string) string {
	return fmt.Sprintf("http://apiserver.%s.svc", host)
}
