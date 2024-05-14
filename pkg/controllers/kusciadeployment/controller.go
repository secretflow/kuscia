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

package kusciadeployment

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	applisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"

	"github.com/secretflow/kuscia/pkg/controllers"
	kusciav1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	maxRetries     = 15
	controllerName = "kusciadeployment-controller"
)

// Controller is the implementation for kusciaDeployment resources.
type Controller struct {
	ctx    context.Context
	cancel context.CancelFunc
	config controllers.ControllerConfig
	// kubeClient is a standard kubernetes clientset
	kubeClient kubernetes.Interface
	// kusciaClient is a clientset for kuscia API group
	kusciaClient kusciaclientset.Interface

	// kusciaDeployment queue
	kdQueue workqueue.RateLimitingInterface

	// shared informer factory of kubernetes, kuscia
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory

	deploymentLister applisters.DeploymentLister
	deploymentSynced cache.InformerSynced
	namespaceLister  corelisters.NamespaceLister
	namespaceSynced  cache.InformerSynced
	serviceLister    corelisters.ServiceLister
	serviceSynced    cache.InformerSynced
	configMapLister  corelisters.ConfigMapLister
	configMapSynced  cache.InformerSynced

	kdLister       kuscialistersv1alpha1.KusciaDeploymentLister
	kdSynced       cache.InformerSynced
	appImageLister kuscialistersv1alpha1.AppImageLister
	appImageSynced cache.InformerSynced
	domainLister   kuscialistersv1alpha1.DomainLister
	domainSynced   cache.InformerSynced
}

// NewController returns a controller instance.
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(config.KubeClient, 5*time.Minute)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(config.KusciaClient, 5*time.Minute)

	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

	controller := &Controller{
		config:                config,
		kubeClient:            config.KubeClient,
		kusciaClient:          config.KusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		deploymentLister:      deploymentInformer.Lister(),
		deploymentSynced:      deploymentInformer.Informer().HasSynced,
		namespaceLister:       nsInformer.Lister(),
		namespaceSynced:       nsInformer.Informer().HasSynced,
		serviceLister:         serviceInformer.Lister(),
		serviceSynced:         serviceInformer.Informer().HasSynced,
		configMapLister:       configMapInformer.Lister(),
		configMapSynced:       configMapInformer.Informer().HasSynced,
		kdLister:              kdInformer.Lister(),
		kdSynced:              kdInformer.Informer().HasSynced,
		appImageLister:        appImageInformer.Lister(),
		appImageSynced:        appImageInformer.Informer().HasSynced,
		domainLister:          domainInformer.Lister(),
		domainSynced:          domainInformer.Informer().HasSynced,
		kdQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "kusciaDeployment"),
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)

	kdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.kusciaDeploymentResourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedOrDeletedKusciaDeployment,
			UpdateFunc: controller.handleUpdatedKusciaDeployment,
			DeleteFunc: controller.handleAddedOrDeletedKusciaDeployment,
		},
	})

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAddedOrDeletedDeployment,
		UpdateFunc: controller.handleUpdatedDeployment,
		DeleteFunc: controller.handleAddedOrDeletedDeployment,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.handleDeletedService,
	})

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.handleDeletedConfigmap,
	})
	return controller
}

// kusciaDeploymentResourceFilter is used to filter resource.
func (c *Controller) kusciaDeploymentResourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch t := obj.(type) {
		case *kusciav1alpha1.KusciaDeployment:
			if t.DeletionTimestamp != nil {
				return false
			}
			if t.Namespace != common.KusciaCrossDomain {
				return false
			}
			return true
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

// handleAddedOrDeletedKusciaDeployment handles added or deleted kusciaDeployment.
func (c *Controller) handleAddedOrDeletedKusciaDeployment(obj interface{}) {
	kd, ok := obj.(*kusciav1alpha1.KusciaDeployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Warnf("Couldn't get object from tombstone %#v", obj)
			return
		}
		kd, ok = tombstone.Obj.(*kusciav1alpha1.KusciaDeployment)
		if !ok {
			nlog.Warnf("Tombstone contained object that is not a KusciaDeployment %#v", obj)
			return
		}
	}
	key, _ := cache.MetaNamespaceKeyFunc(kd)

	c.kdQueue.Add(key)
}

// handleUpdatedKusciaDeployment handles updated kusciaDeployment.
func (c *Controller) handleUpdatedKusciaDeployment(oldObj, newObj interface{}) {
	oldKd, ok := oldObj.(*kusciav1alpha1.KusciaDeployment)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaDeployment", oldObj)
		return
	}

	newKd, ok := newObj.(*kusciav1alpha1.KusciaDeployment)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaDeployment", newObj)
		return
	}

	if oldKd.ResourceVersion == newKd.ResourceVersion {
		return
	}

	key, _ := cache.MetaNamespaceKeyFunc(newKd)

	c.kdQueue.Add(key)
}

// handleAddedOrDeletedDeployment handles added or deleted deployment.
func (c *Controller) handleAddedOrDeletedDeployment(obj interface{}) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Warnf("Couldn't get object from tombstone %#v", obj)
			return
		}
		deployment, ok = tombstone.Obj.(*appsv1.Deployment)
		if !ok {
			nlog.Warnf("Tombstone contained object that is not a deployment %#v", obj)
			return
		}
	}

	kdKey, err := c.getKusciaDeploymentOwnerKey(deployment)
	if err != nil {
		nlog.Warnf("Failed to get deployment %v owner, %v, skip this event", deployment.Name, err)
		return
	}

	if kdKey != "" {
		c.kdQueue.Add(kdKey)
	}
}

// handleUpdatedDeployment handles updated deployment.
func (c *Controller) handleUpdatedDeployment(oldObj, newObj interface{}) {
	oldDeploy, ok := oldObj.(*appsv1.Deployment)
	if !ok {
		nlog.Warnf("Object %#v is not a deployment", oldObj)
		return
	}

	newDeploy, ok := newObj.(*appsv1.Deployment)
	if !ok {
		nlog.Warnf("Object %#v is not a deployment", newObj)
		return
	}

	if oldDeploy.ResourceVersion == newDeploy.ResourceVersion {
		return
	}

	kdKey, err := c.getKusciaDeploymentOwnerKey(newDeploy)
	if err != nil {
		nlog.Warnf("Failed to get deployment %v owner, %v, skip this event", newDeploy.Name, err)
		return
	}

	if kdKey != "" {
		c.kdQueue.Add(kdKey)
	}
}

// handleDeletedService handles deleted deployment.
func (c *Controller) handleDeletedService(obj interface{}) {
	svc, ok := obj.(*v1.Service)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Warnf("Couldn't get object from tombstone %#v", obj)
			return
		}
		svc, ok = tombstone.Obj.(*v1.Service)
		if !ok {
			nlog.Warnf("Tombstone contained object that is not a service %#v", obj)
			return
		}
	}

	kdKey, err := c.getKusciaDeploymentOwnerKey(svc)
	if err != nil {
		nlog.Warnf("Failed to get service %v owner, %v, skip this event", svc.Name, err)
		return
	}

	if kdKey != "" {
		c.kdQueue.Add(kdKey)
	}
}

// handleDeletedService handles deleted deployment.
func (c *Controller) handleDeletedConfigmap(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Warnf("Couldn't get object from tombstone %#v", obj)
			return
		}
		cm, ok = tombstone.Obj.(*v1.ConfigMap)
		if !ok {
			nlog.Warnf("Tombstone contained object that is not a configmap %#v", obj)
			return
		}
	}

	kdKey, err := c.getKusciaDeploymentOwnerKey(cm)
	if err != nil {
		nlog.Warnf("Failed to get configmap %v owner, %v, skip this event", cm.Name, err)
		return
	}

	if kdKey != "" {
		c.kdQueue.Add(kdKey)
	}
}

func (c *Controller) getKusciaDeploymentOwnerKey(obj metav1.Object) (string, error) {
	if obj.GetLabels() != nil && obj.GetLabels()[common.LabelKusciaOwnerNamespace] != "" {
		ownerNamespace := obj.GetLabels()[common.LabelKusciaOwnerNamespace]
		ownerkdName := obj.GetLabels()[common.LabelKusciaDeploymentName]
		if ownerNamespace != "" && ownerkdName != "" {
			kdb, err := c.kdLister.KusciaDeployments(ownerNamespace).Get(ownerkdName)
			if err != nil {
				return "", err
			}
			return ownerNamespace + "/" + kdb.Name, nil
		}
	}
	return "", nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer c.kdQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	nlog.Infof("Starting %v", c.Name())
	c.kusciaInformerFactory.Start(c.ctx.Done())
	c.kubeInformerFactory.Start(c.ctx.Done())

	// Wait for the caches to be synced before starting workers
	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())
	if !cache.WaitForCacheSync(c.ctx.Done(), c.deploymentSynced, c.namespaceSynced, c.serviceSynced, c.configMapSynced, c.kdSynced, c.appImageSynced, c.domainSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Starting %v workers to handle object for %v", workers, c.Name())
	for i := 0; i < workers; i++ {
		go c.runWorker(c.ctx)
	}

	<-c.ctx.Done()
	nlog.Infof("Shutting down %v workers", c.Name())

	return nil
}

// Stop the controller.
func (c *Controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// runWorkerForKdQueue is a long-running function that will continually process item from kd wwork queue.
func (c *Controller) runWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, c.Name(), c.kdQueue, c.syncHandler, maxRetries) {
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("KusciaDeployment %s split failed: %s", key, err)
		return err
	}
	nlog.Infof("KusciaDeployment found: key=%s, ns=%s, name=%s", key, ns, name)
	kusciaDeployment, err := c.kdLister.KusciaDeployments(ns).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("KusciaDeployment %s/%s maybe deleted, delete resources", ns, key)
			if err = c.cleanKusciaDeploymentEvent(ctx, ns, name); err != nil {
				nlog.Errorf("Clean kd %s/%s resources failed: %s", ns, name, err)
				return err
			}
			return nil
		}
		return err
	}

	kd := kusciaDeployment.DeepCopy()
	if err = c.ProcessKusciaDeployment(ctx, kd); err != nil {
		if c.kdQueue.NumRequeues(key) == maxRetries {
			kd.Status.Phase = kusciav1alpha1.KusciaDeploymentPhaseFailed
			kd.Status.Reason = string(retryProcessingFailed)
			kd.Status.Message = fmt.Sprintf("process failed after retrying %v times, %v", maxRetries, err)
			err = c.handleError(ctx, nil, &kusciaDeployment.Status, kd, err)
		}
		return err
	}

	return nil
}

func (c *Controller) cleanKusciaDeploymentEvent(ctx context.Context, ns string, kdName string) error {
	nsRequirement, nsRequirementErr := labels.NewRequirement(common.LabelKusciaOwnerNamespace, selection.Equals, []string{ns})
	kdNameRequirement, kdNameRequirementErr := labels.NewRequirement(common.LabelKusciaDeploymentName, selection.Equals, []string{kdName})
	if nsRequirementErr != nil || kdNameRequirementErr != nil {
		return fmt.Errorf("make requirement failed: ns=%s, kdName=%s", nsRequirementErr, kdNameRequirementErr)
	}
	selector := labels.NewSelector().Add(*nsRequirement, *kdNameRequirement)

	if err := c.deleteService(ctx, selector); err != nil {
		return err
	}

	if err := c.deleteDeployment(ctx, selector); err != nil {
		return err
	}

	if err := c.deleteConfigMap(ctx, selector); err != nil {
		return err
	}

	return nil
}

func (c *Controller) deleteService(ctx context.Context, selector labels.Selector) error {
	services, err := c.serviceLister.List(selector)
	if err != nil {
		return err
	}
	for _, it := range services {
		if err := c.kubeClient.CoreV1().Services(it.Namespace).Delete(ctx, it.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) deleteConfigMap(ctx context.Context, selector labels.Selector) error {
	configs, err := c.configMapLister.List(selector)
	if err != nil {
		return err
	}
	namespaces := make([]string, 0)
	for _, it := range configs {
		namespaces = append(namespaces, it.Namespace)
	}
	for _, ns := range namespaces {
		if err := c.kubeClient.CoreV1().ConfigMaps(ns).DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: selector.String()}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) deleteDeployment(ctx context.Context, selector labels.Selector) error {
	deploys, err := c.deploymentLister.List(selector)
	if err != nil {
		return err
	}
	namespaces := make([]string, 0)
	for _, d := range deploys {
		namespaces = append(namespaces, d.Namespace)
	}
	for _, ns := range namespaces {
		if err := c.kubeClient.AppsV1().Deployments(ns).DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{LabelSelector: selector.String()}); err != nil {
			return err
		}
	}
	return nil
}

// Name returns controller name.
func (c *Controller) Name() string {
	return controllerName
}
