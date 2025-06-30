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

package domain

import (
	"context"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/domain/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	"github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	// maxRetries is the number of times a object will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a object is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15

	controllerName = "domain-controller"
)

const (
	resourceQuotaName = "resource-limitation"
	domainConfigName  = "domain-config"
)

const (
	nodeStatusReady    = "Ready"
	nodeStatusNotReady = "NotReady"
)

// Controller is the implementation for managing domain resources.
type Controller struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	RunMode               common.RunModeType
	Namespace             string
	RootDir               string
	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	resourceQuotaLister   listerscorev1.ResourceQuotaLister
	domainLister          kuscialistersv1alpha1.DomainLister
	namespaceLister       listerscorev1.NamespaceLister
	nodeLister            listerscorev1.NodeLister
	configmapLister       listerscorev1.ConfigMapLister
	roleLister            rbaclisters.RoleLister
	workqueue             workqueue.RateLimitingInterface
	recorder              record.EventRecorder
	cacheSyncs            []cache.InformerSynced
	nodeResourceManager   *NodeResourceManager
}

// NewController returns a controller instance.
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	eventRecorder := config.EventRecorder
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 5*time.Minute)
	resourceQuotaInformer := kubeInformerFactory.Core().V1().ResourceQuotas()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	configmapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	roleInformer := kubeInformerFactory.Rbac().V1().Roles()

	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

	nodeResourceManager := NewNodeResourceManager(
		domainInformer,
		nodeInformer,
		podInformer,
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
	)

	cacheSyncs := []cache.InformerSynced{
		resourceQuotaInformer.Informer().HasSynced,
		domainInformer.Informer().HasSynced,
		namespaceInformer.Informer().HasSynced,
		nodeInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
		configmapInformer.Informer().HasSynced,
		roleInformer.Informer().HasSynced,
	}
	controller := &Controller{
		RunMode:               config.RunMode,
		Namespace:             config.Namespace,
		RootDir:               config.RootDir,
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		resourceQuotaLister:   resourceQuotaInformer.Lister(),
		domainLister:          domainInformer.Lister(),
		namespaceLister:   namespaceInformer.Lister(),
		nodeLister:        nodeInformer.Lister(),
		configmapLister:   configmapInformer.Lister(),
		roleLister:        roleInformer.Lister(),
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "domain"),
		recorder:          eventRecorder,
		cacheSyncs:        cacheSyncs,
		nodeResourceManager: nodeResourceManager,
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)

	controller.addNamespaceEventHandler(namespaceInformer)
	controller.addDomainEventHandler(domainInformer)
	controller.addResourceQuotaEventHandler(resourceQuotaInformer)
	controller.addConfigMapHandler(configmapInformer)
	controller.nodeResourceManager.addPodEventHandler()
	controller.nodeResourceManager.addNodeEventHandler()

	return controller
}
// addNamespaceEventHandler is used to add event handler for namespace informer.
func (c *Controller) addNamespaceEventHandler(nsInformer informerscorev1.NamespaceInformer) {
	_, _ = nsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *apicorev1.Namespace:
				return c.matchLabels(t)
			case cache.DeletedFinalStateUnknown:
				if rq, ok := t.Obj.(*apicorev1.Namespace); ok {
					return c.matchLabels(rq)
				}
				return false
			default:
				return false
			}
		},

		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueNamespace,
			UpdateFunc: func(oldObj, newObj interface{}) {
				newNs, ok := newObj.(*apicorev1.Namespace)
				if !ok {
					nlog.Warnf("Unable convert object %T to Namespace", newNs)
					return
				}
				oldNs, ok := oldObj.(*apicorev1.Namespace)
				if !ok {
					nlog.Warnf("Unable convert object %T to Namespace", oldNs)
					return
				}

				if newNs.ResourceVersion == oldNs.ResourceVersion {
					return
				}
				c.enqueueNamespace(newObj)
			},
			DeleteFunc: c.enqueueNamespace,
		},
	})
}

// addDomainEventHandler is used to add event handler for domain informer.
func (c *Controller) addDomainEventHandler(domainInformer kusciaextv1alpha1.DomainInformer) {
	_, _ = domainInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueDomain,
		UpdateFunc: func(oldObj, newObj interface{}) {
			newDomain, ok := newObj.(*kusciaapisv1alpha1.Domain)
			if !ok {
				nlog.Error("Unable convert object to domain")
				return
			}
			oldDomain, ok := oldObj.(*kusciaapisv1alpha1.Domain)
			if !ok {
				nlog.Error("Unable convert object to domain")
				return
			}

			if newDomain.ResourceVersion == oldDomain.ResourceVersion {
				return
			}
			c.enqueueDomain(newObj)
		},
		DeleteFunc: c.enqueueDomain,
	})
}

// addResourceQuotaEventHandler is used to add event handler for resource quota informer.
func (c *Controller) addResourceQuotaEventHandler(rqInformer informerscorev1.ResourceQuotaInformer) {
	_, _ = rqInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *apicorev1.ResourceQuota:
				return c.matchLabels(t)
			case cache.DeletedFinalStateUnknown:
				if rq, ok := t.Obj.(*apicorev1.ResourceQuota); ok {
					return c.matchLabels(rq)
				}
				return false
			default:
				return false
			}
		},

		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueResourceQuota,
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldRQ, ok := oldObj.(*apicorev1.ResourceQuota)
				if !ok {
					nlog.Error("Unable convert object to resource quota")
					return
				}
				newRQ, ok := newObj.(*apicorev1.ResourceQuota)
				if !ok {
					nlog.Error("Unable convert object to resource quota")
					return
				}
				if oldRQ.ResourceVersion == newRQ.ResourceVersion {
					return
				}
				c.enqueueResourceQuota(newObj)
			},
			DeleteFunc: c.enqueueResourceQuota,
		},
	})
}

// addConfigMapHandler is used to add event handler for configmap informer.
func (c *Controller) addConfigMapHandler(cmInformer informerscorev1.ConfigMapInformer) {
	_, _ = cmInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *apicorev1.ConfigMap:
				return c.matchLabels(t)
			case cache.DeletedFinalStateUnknown:
				if cm, ok := t.Obj.(*apicorev1.ConfigMap); ok {
					return c.matchLabels(cm)
				}
				return false
			default:
				return false
			}
		},

		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueueConfigMap,
			DeleteFunc: c.enqueueConfigMap,
		},
	})
}
// matchLabels is used to filter concerned resource.
func (c *Controller) matchLabels(obj apismetav1.Object) bool {
	if labels := obj.GetLabels(); labels != nil {
		_, ok := labels[common.LabelDomainName]
		if ok {
			return true
		}
	}
	return false
}

// enqueueDomain puts a domain resource onto the workqueue.
// This method should *not* be passed resources of any type other than domain.
func (c *Controller) enqueueDomain(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.workqueue)
}

// enqueueResourceQuota puts a resource quota resource onto the workqueue.
// This method should *not* be passed resources of any type other than resource quota.
func (c *Controller) enqueueResourceQuota(obj interface{}) {
	queue.EnqueueObjectWithKeyNamespace(obj, c.workqueue)
}

// enqueueNamespace puts a namespace resource onto the workqueue.
// This method should *not* be passed resources of any type other than namespace.
func (c *Controller) enqueueNamespace(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.workqueue)
}

// enqueueConfigMap puts a configmap resource onto the workqueue.
// This method should *not* be passed resources of any type other than resource quota.
func (c *Controller) enqueueConfigMap(obj interface{}) {
	queue.EnqueueObjectWithKeyNamespace(obj, c.workqueue)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	nlog.Info("Starting domain controller")
	c.kusciaInformerFactory.Start(c.ctx.Done())
	c.kubeInformerFactory.Start(c.ctx.Done())

	nlog.Info("Waiting for informer cache to sync")
	if !cache.WaitForCacheSync(c.ctx.Done(), c.cacheSyncs...) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("nodeResourceManager start initLocalNodeStatus")
	err := c.nodeResourceManager.initLocalNodeStatus()
	if err != nil {
		nlog.Errorf("nodeResourceManager initLocalNodeStatus failed with %v", err)
		return nil
	}

	nlog.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, c.ctx.Done())
		go wait.Until(c.runPodHandleWorker, time.Second, c.ctx.Done())
		go wait.Until(c.runNodeHandleWorker, time.Second, c.ctx.Done())
	}

	nlog.Info("Starting sync domain status")
	go wait.Until(c.syncDomainStatuses, 10*time.Second, c.ctx.Done())
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
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *Controller) runWorker() {
	for queue.HandleQueueItem(context.Background(), controllerName, c.workqueue, c.syncHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(c.workqueue.Len()))
	}
}

func (c *Controller) runPodHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), controllerName, c.nodeResourceManager.podQueue, c.nodeResourceManager.podHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(c.nodeResourceManager.podQueue.Len()))
	}
}

func (c *Controller) runNodeHandleWorker() {
	for queue.HandleNodeAndPodQueueItem(context.Background(), controllerName, c.nodeResourceManager.nodeQueue, c.nodeResourceManager.nodeHandler, maxRetries) {
		metrics.WorkerQueueSize.Set(float64(c.nodeResourceManager.nodeQueue.Len()))
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the domain resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) (err error) {
	rawDomain, err := c.domainLister.Get(key)
	if err != nil {
		// domain resource is deleted
		if k8serrors.IsNotFound(err) {
			return c.delete(key)
		}
		return err
	}

	domain := rawDomain.DeepCopy()
	scheme.Scheme.Default(domain)

	if _, err = c.namespaceLister.Get(key); err != nil {
		if k8serrors.IsNotFound(err) {
			return c.create(domain)
		}
		return err
	}

	return c.update(domain)
}

// create is used to create resource under domain.
func (c *Controller) create(domain *kusciaapisv1alpha1.Domain) error {
	if err := c.createNamespace(domain); err != nil {
		nlog.Warnf("Create domain %v namespace failed: %v", domain.Name, err.Error())
		return err
	}

	if err := c.createOrUpdateDomainRole(domain); err != nil {
		nlog.Warnf("Create or update domain %v role failed: %v", domain.Name, err.Error())
		return err
	}

	if !isPartner(domain) {
		if err := c.createDomainConfig(domain); err != nil {
			nlog.Warnf("Create domain %v configmap failed: %v", domain.Name, err.Error())
			return err
		}

		if err := c.createResourceQuota(domain); err != nil {
			nlog.Warnf("Create domain %v resource quota failed: %v", domain.Name, err.Error())
			return err
		}
	}

	if shouldCreateOrUpdate(domain) {
		if err := c.createOrUpdateAuth(domain); err != nil {
			nlog.Warnf("Create domain %v auth failed: %v", domain.Name, err.Error())
			return err
		}
	}

	return nil
}

// update is used to update resource under domain.
func (c *Controller) update(domain *kusciaapisv1alpha1.Domain) error {
	if err := c.updateNamespace(domain); err != nil {
		nlog.Warnf("Update domain %v namespace failed: %v", domain.Name, err.Error())
		return err
	}

	if err := c.createOrUpdateDomainRole(domain); err != nil {
		nlog.Warnf("Create or update domain %v role failed: %v", domain.Name, err.Error())
		return err
	}

	if shouldCreateOrUpdate(domain) {
		if err := c.createOrUpdateAuth(domain); err != nil {
			nlog.Warnf("update domain %v auth failed: %v", domain.Name, err.Error())
			return err
		}
		return nil
	}

	if !isPartner(domain) {
		if err := c.createDomainConfig(domain); err != nil {
			nlog.Warnf("Create domain %v configmap failed: %v", domain.Name, err.Error())
			return err
		}

		if err := c.updateResourceQuota(domain); err != nil {
			nlog.Warnf("Update domain %v resource quota failed: %v", domain.Name, err.Error())
			return err
		}

		if err := c.syncDomainStatus(domain); err != nil {
			nlog.Warnf("sync domain %v status failed: %v", domain.Name, err.Error())
			return err
		}
	}
	return nil
}

func (c *Controller) createOrUpdateDomainRole(domain *kusciaapisv1alpha1.Domain) error {
	ownerRef := apismetav1.NewControllerRef(domain, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("Domain"))
	return resources.CreateOrUpdateRole(context.Background(), c.kubeClient, c.roleLister, c.RootDir, domain.Name, ownerRef)
}

// delete is used to delete resource under domain.
func (c *Controller) delete(name string) error {
	if err := c.deleteNamespace(name); err != nil {
		nlog.Errorf("Delete domain %v namespace failed: %v", name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) Name() string {
	return controllerName
}

func isPartner(domain *kusciaapisv1alpha1.Domain) bool {
	if domain.Spec.Role == kusciaapisv1alpha1.Partner {
		return true
	}
	return false
}
