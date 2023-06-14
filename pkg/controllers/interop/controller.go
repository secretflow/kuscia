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
package interop

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/interop/hostresources"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	maxRetries                       = 15
	defaultResync                    = 5 * time.Minute
	cleanupResidualResourcesInterval = 10 * time.Minute
)

const (
	controllerName         = "interop-controller"
	podQueueName           = "interop-controller-pod-queue"
	taskResourceQueueName  = "interop-controller-task-resource-queue"
	interopConfigQueueName = "interop-controller-interop-config-queue"
)

// Controller is the implementation for managing resources.
type Controller struct {
	mu                    sync.Mutex
	ctx                   context.Context
	cancel                context.CancelFunc
	interopConfigInfos    map[string]*interopConfigInfo
	hostResourceManager   hostresources.ResourcesManager
	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kubeInformerFactory   informers.SharedInformerFactory
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	namespaceSynced       cache.InformerSynced
	namespaceLister       listers.NamespaceLister
	podSynced             cache.InformerSynced
	podLister             listers.PodLister
	serviceSynced         cache.InformerSynced
	serviceLister         listers.ServiceLister
	configMapSynced       cache.InformerSynced
	configMapLister       listers.ConfigMapLister
	taskResourceSynced    cache.InformerSynced
	taskResourceLister    kuscialistersv1alpha1.TaskResourceLister
	interopConfigSynced   cache.InformerSynced
	interopConfigLister   kuscialistersv1alpha1.InteropConfigLister
	interopConfigQueue    workqueue.RateLimitingInterface
	podQueue              workqueue.RateLimitingInterface
	taskResourceQueue     workqueue.RateLimitingInterface
}

type interopConfigInfo struct {
	host    string
	members []string
}

// NewController returns a controller instance.
func NewController(ctx context.Context, kubeClient kubernetes.Interface, kusciaClient kusciaclientset.Interface, eventRecorder record.EventRecorder) controllers.IController {
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, defaultResync)
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, defaultResync)
	interopConfigInformer := kusciaInformerFactory.Kuscia().V1alpha1().InteropConfigs()
	taskResourceInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	hMgrOpt := &hostresources.Options{
		MemberKubeClient:      kubeClient,
		MemberKusciaClient:    kusciaClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   serviceInformer.Lister(),
		MemberConfigMapLister: configMapInformer.Lister(),
		MemberTrLister:        taskResourceInformer.Lister(),
	}

	controller := &Controller{
		interopConfigInfos:    make(map[string]*interopConfigInfo),
		hostResourceManager:   hostresources.NewHostResourcesManager(hMgrOpt),
		kubeClient:            kubeClient,
		kusciaClient:          kusciaClient,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaInformerFactory: kusciaInformerFactory,
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		namespaceLister:       namespaceInformer.Lister(),
		podSynced:             podInformer.Informer().HasSynced,
		podLister:             podInformer.Lister(),
		serviceSynced:         serviceInformer.Informer().HasSynced,
		serviceLister:         serviceInformer.Lister(),
		configMapSynced:       configMapInformer.Informer().HasSynced,
		configMapLister:       configMapInformer.Lister(),
		taskResourceSynced:    taskResourceInformer.Informer().HasSynced,
		taskResourceLister:    taskResourceInformer.Lister(),
		interopConfigSynced:   interopConfigInformer.Informer().HasSynced,
		interopConfigLister:   interopConfigInformer.Lister(),
		interopConfigQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), interopConfigQueueName),
		podQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), podQueueName),
		taskResourceQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), taskResourceQueueName),
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)

	interopConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleAddedorDeletedInteropConfig,
		UpdateFunc: controller.handleUpdatedInteropConfig,
		DeleteFunc: controller.handleAddedorDeletedInteropConfig,
	})

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedPod,
			UpdateFunc: controller.handleUpdatedPod,
			DeleteFunc: controller.handleDeletedPod,
		},
	})

	taskResourceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.handleAddedTaskResource,
			UpdateFunc: controller.handleUpdatedTaskResource,
			DeleteFunc: controller.handleDeletedTaskResource,
		},
	})

	return controller
}

// resourceFilter is used to filter specific resources.
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
	ns, err := c.namespaceLister.Get(obj.GetNamespace())
	if err != nil {
		if k8serrors.IsNotFound(err) {
			ns, err = c.kubeClient.CoreV1().Namespaces().Get(context.Background(), obj.GetNamespace(), metav1.GetOptions{})
		}

		if err != nil {
			nlog.Warnf("Failed to get object %v namespace %v, %v, skip handling it", obj.GetName(), obj.GetNamespace(), err.Error())
			return false
		}
	}

	if ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
		return false
	}

	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	initiator := labels[common.LabelTaskInitiator]
	if initiator == "" || initiator == ns.Name {
		return false
	}

	rv := labels[common.LabelResourceVersionUnderHostCluster]
	if rv == "" {
		return false
	}

	return true
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	nlog.Info("Start interop controller")
	defer func() {
		c.interopConfigQueue.ShutDown()
		c.podQueue.ShutDown()
		c.taskResourceQueue.ShutDown()
	}()

	c.hostResourceManager.SetWorkers(workers)

	c.kubeInformerFactory.Start(c.ctx.Done())
	c.kusciaInformerFactory.Start(c.ctx.Done())

	nlog.Info("Start waiting for cache sync for interop controller")
	if !cache.WaitForCacheSync(c.ctx.Done(), c.interopConfigSynced, c.namespaceSynced, c.podSynced, c.serviceSynced, c.configMapSynced, c.taskResourceSynced) {
		return fmt.Errorf("failed to wait for cache sync for interop controller")
	}
	nlog.Info("Finish waiting for cache sync for interop controller")
	c.registerInteropConfigs()

	go wait.Until(c.cleanupResidualResources, cleanupResidualResourcesInterval, c.ctx.Done())

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(c.ctx, c.runInteropConfigWorker, time.Second)
		go wait.UntilWithContext(c.ctx, c.runPodWorker, time.Second)
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

// cleanupResidualResources is used to clean up residual resources.
func (c *Controller) cleanupResidualResources() {
	reqForInitiator, err := labels.NewRequirement(common.LabelTaskInitiator, selection.Exists, nil)
	if err != nil {
		nlog.Warnf("New label requirement failed, %v", err.Error())
		return
	}

	reqForResourceVersion, err := labels.NewRequirement(common.LabelResourceVersionUnderHostCluster, selection.Exists, nil)
	if err != nil {
		nlog.Warnf("New label requirement failed, %v", err.Error())
		return
	}

	selector := labels.NewSelector().Add(*reqForInitiator, *reqForResourceVersion)
	c.cleanupResidualPods(selector)
	c.cleanupResidualServices(selector)
	c.cleanupResidualConfigmaps(selector)
	c.cleanupResidualTaskResources(selector)
}

// cleanupResidualPods is used to clean up residual pods.
func (c *Controller) cleanupResidualPods(selector labels.Selector) {
	pods, err := c.podLister.List(selector)
	if err != nil {
		nlog.Warnf("List pods by label selector %v failed, %v", selector.String(), err.Error())
		return
	}

	for _, pod := range pods {
		initiator := pod.Labels[common.LabelTaskInitiator]
		ra := c.hostResourceManager.GetHostResourceAccessor(initiator, pod.Namespace)
		if ra == nil {
			nlog.Infof("Delete residual pod %v/%v", pod.Namespace, pod.Name)
			if err = c.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual pod %v/%v failed, %v", pod.Namespace, pod.Name, err.Error())
			}
			continue
		}

		if !ra.HasSynced() {
			nlog.Infof("Host %v resource accessor has not synced, skip cleaning up residual pods", initiator)
			continue
		}

		if _, err = ra.HostPodLister().Pods(pod.Namespace).Get(pod.Name); k8serrors.IsNotFound(err) {
			nlog.Infof("Delete residual pod %v/%v", pod.Namespace, pod.Name)
			if err = c.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual pod %v/%v failed, %v", pod.Namespace, pod.Name, err.Error())
			}
		}
	}
}

// cleanupResidualServices is used to clean up residual services.
func (c *Controller) cleanupResidualServices(selector labels.Selector) {
	services, err := c.serviceLister.List(selector)
	if err != nil {
		nlog.Warnf("List services by label selector %v failed, %v", selector.String(), err.Error())
		return
	}

	for _, service := range services {
		initiator := service.Labels[common.LabelTaskInitiator]
		ra := c.hostResourceManager.GetHostResourceAccessor(initiator, service.Namespace)
		if ra == nil {
			nlog.Infof("Delete residual service %v/%v", service.Namespace, service.Name)
			if err = c.kubeClient.CoreV1().Services(service.Namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual service %v/%v failed, %v", service.Namespace, service.Name, err.Error())
			}
			continue
		}

		if !ra.HasSynced() {
			nlog.Infof("Host %v resource accessor has not synced, skip cleaning up residual services", initiator)
			continue
		}

		if _, err = ra.HostServiceLister().Services(service.Namespace).Get(service.Name); k8serrors.IsNotFound(err) {
			nlog.Infof("Delete residual service %v/%v", service.Namespace, service.Name)
			if err = c.kubeClient.CoreV1().Services(service.Namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual service %v/%v failed, %v", service.Namespace, service.Name, err.Error())
			}
		}
	}
}

// cleanupResidualConfigmaps is used to clean up residual configmaps.
func (c *Controller) cleanupResidualConfigmaps(selector labels.Selector) {
	configmaps, err := c.configMapLister.List(selector)
	if err != nil {
		nlog.Warnf("List configmaps by label selector %v failed, %v", selector.String(), err.Error())
		return
	}

	for _, configmap := range configmaps {
		initiator := configmap.Labels[common.LabelTaskInitiator]
		ra := c.hostResourceManager.GetHostResourceAccessor(initiator, configmap.Namespace)
		if ra == nil {
			nlog.Infof("Delete residual configmap %v/%v", configmap.Namespace, configmap.Name)
			if err = c.kubeClient.CoreV1().ConfigMaps(configmap.Namespace).Delete(context.Background(), configmap.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual configmap %v/%v failed, %v", configmap.Namespace, configmap.Name, err.Error())
			}
			continue
		}

		if !ra.HasSynced() {
			nlog.Infof("Host %v resource accessor has not synced, skip cleaning up residual configmaps", initiator)
			continue
		}

		if _, err = ra.HostConfigMapLister().ConfigMaps(configmap.Namespace).Get(configmap.Name); k8serrors.IsNotFound(err) {
			nlog.Infof("Delete residual configmap %v/%v", configmap.Namespace, configmap.Name)
			if err = c.kubeClient.CoreV1().ConfigMaps(configmap.Namespace).Delete(context.Background(), configmap.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual configmap %v/%v failed, %v", configmap.Namespace, configmap.Name, err.Error())
			}
		}
	}
}

// cleanupResidualTaskResources is used to clean up residual taskresources.
func (c *Controller) cleanupResidualTaskResources(selector labels.Selector) {
	trs, err := c.taskResourceLister.List(selector)
	if err != nil {
		nlog.Warnf("List task resources by label selector %v failed, %v", selector.String(), err.Error())
		return
	}

	for _, tr := range trs {
		initiator := tr.Labels[common.LabelTaskInitiator]
		ra := c.hostResourceManager.GetHostResourceAccessor(initiator, tr.Namespace)
		if ra == nil {
			nlog.Infof("Delete residual task resource %v/%v", tr.Namespace, tr.Name)
			if err = c.kusciaClient.KusciaV1alpha1().TaskResources(tr.Namespace).Delete(context.Background(), tr.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual task resource %v/%v failed, %v", tr.Namespace, tr.Name, err.Error())
			}
			continue
		}

		if !ra.HasSynced() {
			nlog.Infof("Host %v resource accessor has not synced, skip cleaning up residual task resources", initiator)
			continue
		}

		if _, err = ra.HostTaskResourceLister().TaskResources(tr.Namespace).Get(tr.Name); k8serrors.IsNotFound(err) {
			nlog.Infof("Delete residual task resource %v/%v", tr.Namespace, tr.Name)
			if err = c.kusciaClient.KusciaV1alpha1().TaskResources(tr.Namespace).Delete(context.Background(), tr.Name, metav1.DeleteOptions{}); err != nil {
				nlog.Warnf("Delete residual task resource %v/%v failed, %v", tr.Namespace, tr.Name, err.Error())
			}
		}
	}
}

// Name returns the controller name.
func (c *Controller) Name() string {
	return controllerName
}

// CheckCRDExists is used to check if crd exist.
func CheckCRDExists(ctx context.Context, extensionClient apiextensionsclientset.Interface) error {
	if _, err := extensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, controllers.CRDInteropConfigsName, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}
