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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
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
	maxRetries    = 15
	defaultResync = 5 * time.Minute
)

// ResourcesAccessor is an interface to get host resources controller related information.
type ResourcesAccessor interface {
	HasSynced() bool
	HostKubeClient() kubernetes.Interface
	HostKusciaClient() kusciaclientset.Interface
	HostPodLister() listers.PodLister
	HostServiceLister() listers.ServiceLister
	HostConfigMapLister() listers.ConfigMapLister
	HostTaskResourceLister() kuscialistersv1alpha1.TaskResourceLister
	EnqueueTaskResource(string)
	EnqueuePod(string)
	EnqueueService(string)
}

// hostResourcesController is used to manage host resources.
type hostResourcesController struct {
	host                      string
	member                    string
	hasSynced                 bool
	stopCh                    chan struct{}
	hostKubeClient            kubernetes.Interface
	hostKusciaClient          kusciaclientset.Interface
	hostKubeInformerFactory   informers.SharedInformerFactory
	hostKusciaInformerFactory kusciainformers.SharedInformerFactory
	hostPodSynced             cache.InformerSynced
	hostPodLister             listers.PodLister
	hostServiceSynced         cache.InformerSynced
	hostServiceLister         listers.ServiceLister
	hostConfigMapSynced       cache.InformerSynced
	hostConfigMapLister       listers.ConfigMapLister
	hostTrSynced              cache.InformerSynced
	hostTrLister              kuscialistersv1alpha1.TaskResourceLister
	memberKubeClient          kubernetes.Interface
	memberKusciaClient        kusciaclientset.Interface
	memberPodLister           listers.PodLister
	memberServiceLister       listers.ServiceLister
	memberConfigMapLister     listers.ConfigMapLister
	memberTrLister            kuscialistersv1alpha1.TaskResourceLister
	podsQueue                 workqueue.RateLimitingInterface
	servicesQueue             workqueue.RateLimitingInterface
	configMapsQueue           workqueue.RateLimitingInterface
	taskResourcesQueue        workqueue.RateLimitingInterface
}

// hostResourcesControllerOptions defines some options for host resources controller.
type hostResourcesControllerOptions struct {
	host                  string
	member                string
	memberKubeClient      kubernetes.Interface
	memberKusciaClient    kusciaclientset.Interface
	memberPodLister       listers.PodLister
	memberServiceLister   listers.ServiceLister
	memberConfigMapLister listers.ConfigMapLister
	memberTrLister        kuscialistersv1alpha1.TaskResourceLister
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

	hKubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(clients.KubeClient, defaultResync, informers.WithNamespace(member))
	hPodInformer := hKubeInformerFactory.Core().V1().Pods()
	hServiceInformer := hKubeInformerFactory.Core().V1().Services()
	hConfigMapInformer := hKubeInformerFactory.Core().V1().ConfigMaps()

	hKusciaInformerFactory := kusciainformers.NewSharedInformerFactoryWithOptions(clients.KusciaClient, defaultResync, kusciainformers.WithNamespace(member))
	hTaskResourceInformer := hKusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	stopCh := make(chan struct{})
	hrc := &hostResourcesController{
		stopCh:                    stopCh,
		host:                      host,
		member:                    member,
		hostKubeClient:            clients.KubeClient,
		hostKusciaClient:          clients.KusciaClient,
		memberKubeClient:          opts.memberKubeClient,
		memberKusciaClient:        opts.memberKusciaClient,
		hostKubeInformerFactory:   hKubeInformerFactory,
		hostKusciaInformerFactory: hKusciaInformerFactory,
		hostPodSynced:             hPodInformer.Informer().HasSynced,
		hostPodLister:             hPodInformer.Lister(),
		hostServiceSynced:         hServiceInformer.Informer().HasSynced,
		hostServiceLister:         hServiceInformer.Lister(),
		hostConfigMapSynced:       hConfigMapInformer.Informer().HasSynced,
		hostConfigMapLister:       hConfigMapInformer.Lister(),
		hostTrSynced:              hTaskResourceInformer.Informer().HasSynced,
		hostTrLister:              hTaskResourceInformer.Lister(),
		memberPodLister:           opts.memberPodLister,
		memberServiceLister:       opts.memberServiceLister,
		memberConfigMapLister:     opts.memberConfigMapLister,
		memberTrLister:            opts.memberTrLister,
		podsQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf(hostPodsQueueName, host, member)),
		servicesQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf(hostServicesQueueName, host, member)),
		configMapsQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf(hostConfigMapsQueueName, host, member)),
		taskResourcesQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf(hostTaskResourcesQueueName, host, member)),
	}

	hPodInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedOrDeletedPod,
			UpdateFunc: hrc.handleUpdatedPod,
			DeleteFunc: hrc.handleAddedOrDeletedPod,
		},
	})

	hServiceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedOrDeletedService,
			UpdateFunc: hrc.handleUpdatedService,
			DeleteFunc: hrc.handleAddedOrDeletedService,
		},
	})

	hConfigMapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedOrDeletedConfigMap,
			UpdateFunc: hrc.handleUpdatedConfigMap,
			DeleteFunc: hrc.handleAddedOrDeletedConfigMap,
		},
	})

	hTaskResourceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: hrc.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    hrc.handleAddedOrDeletedTaskResource,
			UpdateFunc: hrc.handleUpdatedTaskResource,
			DeleteFunc: hrc.handleAddedOrDeletedTaskResource,
		},
	})

	return hrc, nil
}

// resourceFilter is used to filter resource.
func (c *hostResourcesController) resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch t := obj.(type) {
		case *corev1.Pod:
			return c.matchLabels(t)
		case *corev1.Service:
			return c.matchLabels(t)
		case *corev1.ConfigMap:
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
func (c *hostResourcesController) matchLabels(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	if labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnKuscia) {
		return false
	}

	value, exist := labels[common.LabelInitiator]
	if !exist || value != c.host {
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
	c.hostKubeInformerFactory.Start(c.stopCh)
	c.hostKusciaInformerFactory.Start(c.stopCh)

	nlog.Infof("Start waiting for cache to sync for %v/%v resource controller", c.host, c.member)
	if ok := cache.WaitForCacheSync(c.stopCh, c.hostPodSynced, c.hostServiceSynced, c.hostConfigMapSynced, c.hostTrSynced); !ok {
		return fmt.Errorf("failed to wait for %v/%v cache to sync for resource controller", c.host, c.member)
	}
	nlog.Infof("Finish waiting for cache to sync for %v/%v resource controller", c.host, c.member)

	c.hasSynced = true

	nlog.Infof("Run workers for %v/%v resource controller", c.host, c.member)
	for i := 0; i < workers; i++ {
		go wait.Until(c.runPodWorker, time.Second, c.stopCh)
		go wait.Until(c.runServiceWorker, time.Second, c.stopCh)
		go wait.Until(c.runConfigMapWorker, time.Second, c.stopCh)
		go wait.Until(c.runTaskResourceWorker, time.Second, c.stopCh)
	}
	return nil
}

// stop is used to stop the host resource controller.
func (c *hostResourcesController) stop() {
	c.podsQueue.ShutDown()
	c.servicesQueue.ShutDown()
	c.configMapsQueue.ShutDown()
	c.taskResourcesQueue.ShutDown()
	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}
	nlog.Infof("%v/%v resource controller stopped", c.host, c.member)
}

func (c *hostResourcesController) HasSynced() bool {
	return c.hasSynced
}

// HostKubeClient returns kubeClient of host cluster.
func (c *hostResourcesController) HostKubeClient() kubernetes.Interface {
	return c.hostKubeClient
}

// HostKusciaClient returns KusciaClient of host cluster.
func (c *hostResourcesController) HostKusciaClient() kusciaclientset.Interface {
	return c.hostKusciaClient
}

// HostPodLister returns podLister of host cluster.
func (c *hostResourcesController) HostPodLister() listers.PodLister {
	return c.hostPodLister
}

// HostServiceLister returns serviceLister of host cluster.
func (c *hostResourcesController) HostServiceLister() listers.ServiceLister {
	return c.hostServiceLister
}

// HostConfigMapLister returns configmapLister of host cluster.
func (c *hostResourcesController) HostConfigMapLister() listers.ConfigMapLister {
	return c.hostConfigMapLister
}

// HostTaskResourceLister returns taskResourceLister of host cluster.
func (c *hostResourcesController) HostTaskResourceLister() kuscialistersv1alpha1.TaskResourceLister {
	return c.hostTrLister
}

// EnqueueTaskResource is used to put task resource key into host task resources queue.
func (c *hostResourcesController) EnqueueTaskResource(key string) {
	c.taskResourcesQueue.Add(key)
}

// EnqueuePod is used to put pod key into host pods queue.
func (c *hostResourcesController) EnqueuePod(key string) {
	c.podsQueue.Add(key)
}

// EnqueueService is used to put service key into host services queue.
func (c *hostResourcesController) EnqueueService(key string) {
	c.servicesQueue.Add(key)
}

// GetHostClient returns kubeClient and kusciaClient of host cluster.
var GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
	return kubeconfig.CreateClientSetsFromToken(token, masterURL)
}

// getAPIServerURLForHostCluster returns api server url for host cluster.
func getAPIServerURLForHostCluster(host string) string {
	return fmt.Sprintf("http://apiserver.%s.svc", host)
}

// generateQueueName is used to generate queue name.
func generateQueueName(name, host, member string) string {
	return fmt.Sprintf(name, host, member)
}

// getNamespaceAndNameFromKey is used to get namespace and name from key.
func getNamespaceAndNameFromKey(key, member string) (string, string, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return namespace, name, fmt.Errorf("split meta namespace key %v failed, %v", key, err)
	}

	if namespace != member {
		return namespace, name, fmt.Errorf("%v namespace is not equal to member %v", key, member)
	}
	return namespace, name, nil
}
