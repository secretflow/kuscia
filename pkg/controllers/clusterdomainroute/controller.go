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
package clusterdomainroute

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/domainroute"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	clusterDomainRouteSyncPeriod = 2 * time.Minute
	controllerName               = "cluster-domain-route-controller"
)

type controller struct {
	ctx    context.Context
	cancel context.CancelFunc

	Namespace string

	clusterDomainRouteLister       kuscialistersv1alpha1.ClusterDomainRouteLister
	clusterDomainRouteListerSynced cache.InformerSynced
	domainLister                   kuscialistersv1alpha1.DomainLister
	domainListerSynced             cache.InformerSynced
	domainRouteLister              kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced        cache.InformerSynced
	gatewayLister                  kuscialistersv1alpha1.GatewayLister
	gatewayListerSynced            cache.InformerSynced
	interopLister                  kuscialistersv1alpha1.InteropConfigLister
	interopListerSynced            cache.InformerSynced

	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory informers.SharedInformerFactory

	clusterDomainRouteWorkqueue workqueue.RateLimitingInterface
}

func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kusciaInformerFactory := informers.NewSharedInformerFactory(config.KusciaClient, clusterDomainRouteSyncPeriod)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	clusterDomainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().ClusterDomainRoutes()
	domainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()
	gatewayInformer := kusciaInformerFactory.Kuscia().V1alpha1().Gateways()
	interopInformer := kusciaInformerFactory.Kuscia().V1alpha1().InteropConfigs()

	cc := &controller{
		kusciaInformerFactory:          kusciaInformerFactory,
		kusciaClient:                   config.KusciaClient,
		Namespace:                      config.Namespace,
		domainLister:                   domainInformer.Lister(),
		domainListerSynced:             domainInformer.Informer().HasSynced,
		clusterDomainRouteLister:       clusterDomainRouteInformer.Lister(),
		clusterDomainRouteListerSynced: clusterDomainRouteInformer.Informer().HasSynced,
		gatewayLister:                  gatewayInformer.Lister(),
		gatewayListerSynced:            gatewayInformer.Informer().HasSynced,
		domainRouteLister:              domainRouteInformer.Lister(),
		domainRouteListerSynced:        domainRouteInformer.Informer().HasSynced,
		interopLister:                  interopInformer.Lister(),
		interopListerSynced:            interopInformer.Informer().HasSynced,
		clusterDomainRouteWorkqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ClusterDomainRoutes"),
	}
	cc.ctx, cc.cancel = context.WithCancel(ctx)
	clusterDomainRouteInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(newOne interface{}) {
				newCdr, ok := newOne.(*kusciaapisv1alpha1.ClusterDomainRoute)
				if !ok {
					return
				}
				nlog.Debugf("Found new clusterdomain(%s)", newCdr.Name)
				cc.enqueueClusterDomainRoute(newCdr)
			},
			UpdateFunc: func(_, newOne interface{}) {
				newCdr, ok := newOne.(*kusciaapisv1alpha1.ClusterDomainRoute)
				if !ok {
					return
				}

				nlog.Debugf("Found clusterdomainroute(%s) update", newCdr.Name)
				cc.enqueueClusterDomainRoute(newCdr)
			},
			DeleteFunc: func(obj interface{}) {
				newCdr, ok := obj.(*kusciaapisv1alpha1.ClusterDomainRoute)
				if !ok {
					return
				}
				nlog.Debugf("Found clusterdomain(%s) delete", newCdr.Name)
			},
		},
		clusterDomainRouteSyncPeriod,
	)
	domainInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newOne, ok := obj.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				nlog.Debugf("Sync clusterdomainroute because found new domain(%s)", newOne.Name)
				cc.syncClusterDomainRouteByDomainName(common.KusciaSourceKey, newOne.Name)
				cc.syncClusterDomainRouteByDomainName(common.KusciaDestinationKey, newOne.Name)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newOne, ok := newObj.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				oldOne, ok := oldObj.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				if oldOne.Spec.Cert == newOne.Spec.Cert {
					return
				}
				nlog.Debugf("Sync clusterdomainroute because found domain(%s) update", newOne.Name)
				cc.syncClusterDomainRouteByDomainName(common.KusciaSourceKey, newOne.Name)
				cc.syncClusterDomainRouteByDomainName(common.KusciaDestinationKey, newOne.Name)
			},
		},
		0,
	)
	domainRouteInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newOne, ok := obj.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}

				if len(newOne.OwnerReferences) > 0 && newOne.OwnerReferences[0].Kind == "ClusterDomainRoute" {
					nlog.Debugf("Sync clusterdomainroute because found new domainroute(%s)", newOne.Name)
					cc.enqueueClusterDomainRoute(newOne)
				}
			},
			UpdateFunc: func(_, newObj interface{}) {
				newOne, ok := newObj.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}

				if len(newOne.OwnerReferences) > 0 && newOne.OwnerReferences[0].Kind == "ClusterDomainRoute" {
					nlog.Debugf("Sync clusterdomainroute because found domainroute(%s) update", newOne.Name)
					cc.enqueueClusterDomainRoute(newOne)
				}
			},
			DeleteFunc: func(obj interface{}) {
				cc.enqueueClusterDomainRoute(obj)
			},
		},
		0,
	)
	return cc
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *controller) runWorker(ctx context.Context) {
	for queue.HandleQueueItemWithAlwaysRetry(ctx, controllerName, c.clusterDomainRouteWorkqueue, c.syncHandler) {
	}
}

func needDeleteDr(cdr *kusciaapisv1alpha1.ClusterDomainRoute, dr *kusciaapisv1alpha1.DomainRoute) bool {
	if !reflect.DeepEqual(cdr.Spec.Endpoint, dr.Spec.Endpoint) {
		return true
	}

	if cdr.Spec.TokenConfig == nil || dr.Spec.TokenConfig == nil {
		return cdr.Spec.TokenConfig != dr.Spec.TokenConfig
	}
	if cdr.Spec.TokenConfig.DestinationPublicKey != dr.Spec.TokenConfig.DestinationPublicKey {
		return true
	}
	if cdr.Spec.TokenConfig.SourcePublicKey != dr.Spec.TokenConfig.SourcePublicKey {
		return true
	}

	return false
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ClusterDomainRoute resource
// with the current status of the resource.
func (c *controller) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the ClusterDomainRoute resource with this namespace/name
	cdr, err := c.clusterDomainRouteLister.Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Errorf("clusterdomainroute '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}

	// Check domain role
	sourceRole, destRole, err := c.getDomainRole(cdr)
	if err != nil {
		return err
	}

	appendLabels := make(map[string]string, 0)
	appendLabels[common.KusciaSourceKey] = cdr.Spec.Source
	appendLabels[common.KusciaDestinationKey] = cdr.Spec.Destination
	if sourceRole == kusciaapisv1alpha1.Partner {
		appendLabels[common.LabelDomainRoutePartner] = cdr.Spec.Source
	} else if destRole == kusciaapisv1alpha1.Partner {
		appendLabels[common.LabelDomainRoutePartner] = cdr.Spec.Destination
	}

	// Update label must be first.
	if hasUpdate, err := c.updateLabel(ctx, cdr, appendLabels, nil); err != nil || hasUpdate {
		return err
	}

	if hasUpdate, err := c.syncDomainPubKey(ctx, cdr); err != nil || hasUpdate {
		return err
	}

	if err := domainroute.DoValidate(&cdr.Spec.DomainRouteSpec); err != nil {
		nlog.Errorf("clusterdomainroute %s doValidate error:%v", cdr.Name, err)
		return err
	}

	var srcdr, destdr *kusciaapisv1alpha1.DomainRoute
	drName := fmt.Sprintf("%s-%s", cdr.Spec.Source, cdr.Spec.Destination)
	if sourceRole != kusciaapisv1alpha1.Partner {
		// Create domainroute in source namespace
		if hasUpdate, err := c.checkDomainRoute(ctx, cdr, cdr.Spec.Source, drName); err != nil || hasUpdate {
			return err
		}
		if srcdr, err = c.domainRouteLister.DomainRoutes(cdr.Spec.Source).Get(drName); err != nil {
			return err
		}
	}

	if destRole != kusciaapisv1alpha1.Partner {
		// Create domainroute in destination namespace
		if hasUpdate, err := c.checkDomainRoute(ctx, cdr, cdr.Spec.Destination, drName); err != nil || hasUpdate {
			return err
		}
		if destdr, err = c.domainRouteLister.DomainRoutes(cdr.Spec.Destination).Get(drName); err != nil {
			return err
		}
	}

	if hasUpdate, err := c.syncStatusFromDomainroute(cdr, srcdr, destdr); err != nil || hasUpdate {
		return err
	}

	return c.checkInteropConfig(ctx, cdr, sourceRole, destRole)
}

func (c *controller) enqueueClusterDomainRoute(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.clusterDomainRouteWorkqueue)
}

func (c *controller) syncClusterDomainRouteByDomainName(key, domainName string) {
	cdrReqSrc, _ := labels.NewRequirement(key, selection.Equals, []string{domainName})
	cdrs, err := c.clusterDomainRouteLister.List(labels.NewSelector().Add(*cdrReqSrc))
	if err != nil {
		nlog.Error(err)
		return
	}

	for _, cdr := range cdrs {
		nlog.Infof("Enqueue clusterdomainroute %s, because domain %s update", cdr.Name, domainName)
		c.enqueueClusterDomainRoute(cdr)
	}
}

func (c *controller) Run(threadiness int) error {
	defer utilruntime.HandleCrash()
	defer c.clusterDomainRouteWorkqueue.ShutDown()
	// Start the informer factories to begin populating the informer caches
	nlog.Info("Starting ClusterDomainRoute controller")
	c.kusciaInformerFactory.Start(c.ctx.Done())

	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.domainListerSynced, c.clusterDomainRouteListerSynced,
		c.domainRouteListerSynced, c.gatewayListerSynced, c.interopListerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go wait.Until(func() {
		c.Monitorcdrstatus(c.ctx)
	}, time.Second*30, c.ctx.Done())
	// Launch two workers to process ClusterDomainRoute resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			c.runWorker(c.ctx)
		}, time.Second, c.ctx.Done())
	}

	nlog.Info("Started ClusterDomainRoute controller")
	<-c.ctx.Done()
	return nil
}

func (c *controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *controller) Name() string {
	return controllerName
}

func (c *controller) updateLabel(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, addLabels map[string]string,
	removeLabels map[string]bool) (bool, error) {
	var err error

	needUpdateLabel := func() bool {
		if cdr.Labels == nil {
			return true
		}

		for k, v := range addLabels {
			if oldVal, exist := cdr.Labels[k]; !exist || oldVal != v {
				return true
			}
		}

		for k := range removeLabels {
			if _, exist := cdr.Labels[k]; exist {
				return true
			}
		}

		return false
	}

	if !needUpdateLabel() {
		return false, nil
	}

	cdrCopy := cdr.DeepCopy()
	cdrCopy.Labels = make(map[string]string, 0)

	// copy old labels exclude needRemoved Labels
	for k, v := range cdr.Labels {
		needRemove := false
		if removeLabels != nil {
			_, needRemove = removeLabels[k]
		}
		if !needRemove {
			cdrCopy.Labels[k] = v
		}
	}

	// assure to be added labels
	cdrCopy.Labels[common.KusciaSourceKey] = cdr.Spec.Source
	cdrCopy.Labels[common.KusciaDestinationKey] = cdr.Spec.Destination

	// add new labels
	for k, v := range addLabels {
		cdrCopy.Labels[k] = v
	}

	_, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{})
	if err != nil && !k8serrors.IsConflict(err) {
		nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdrCopy.Spec.Source, cdrCopy.Spec.Destination, err.Error())
		return true, err
	}
	if err == nil {
		nlog.Infof("ClusterDomainRoute %s updateLabel", cdr.Name)
	}

	return true, nil
}

func IsReady(status *kusciaapisv1alpha1.ClusterDomainRouteStatus) bool {
	for _, cond := range status.Conditions {
		if cond.Type == kusciaapisv1alpha1.ClusterDomainRouteReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
