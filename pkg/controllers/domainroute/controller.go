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
package domainroute

import (
	"context"
	"fmt"
	mrand "math/rand"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	domainRouteSyncPeriod = 2 * time.Minute
	controllerName        = "domain-route-controller"
	errErrResourceExists  = "ErrResourceExists"
)

type controller struct {
	ctx    context.Context
	cancel context.CancelFunc

	domainRouteLister       kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced cache.InformerSynced
	gatewayLister           kuscialistersv1alpha1.GatewayLister
	gatewayListerSynced     cache.InformerSynced

	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory informers.SharedInformerFactory

	domainRouteWorkqueue workqueue.RateLimitingInterface
}

// NewController returns a new sample controller
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, domainRouteSyncPeriod)
	gatewayInformer := kusciaInformerFactory.Kuscia().V1alpha1().Gateways()
	domainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()
	c := &controller{
		kusciaClient:            kusciaClient,
		kusciaInformerFactory:   kusciaInformerFactory,
		gatewayLister:           gatewayInformer.Lister(),
		gatewayListerSynced:     gatewayInformer.Informer().HasSynced,
		domainRouteLister:       domainRouteInformer.Lister(),
		domainRouteListerSynced: domainRouteInformer.Informer().HasSynced,
		domainRouteWorkqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DomainRoutes"),
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	gatewayInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(newOne interface{}) {
				newGateway, ok := newOne.(*kusciaapisv1alpha1.Gateway)
				if !ok {
					return
				}
				nlog.Debugf("Syncing DomainRoute because found new Gateway(%s)", newGateway.Namespace)
				c.syncDomainRouteByNamespace(newGateway.Namespace)
			},
		},
		0,
	)

	// Set up an event handler for when DomainRoute resources change
	domainRouteInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(newone interface{}) {
				newDr, ok := newone.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				nlog.Debugf("Found DomainRoute(%s/%s) add", newDr.Namespace, newDr.Name)
				c.enqueueDomainRoute(newone)
			},
			// AddFunc is unnecessary. Clusterdomainroute's handler will create it and continue to process.
			UpdateFunc: func(_, newone interface{}) {
				newDr, ok := newone.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				nlog.Debugf("Found DomainRoute(%s/%s) update", newDr.Namespace, newDr.Name)
				c.enqueueDomainRoute(newone)
			},
			DeleteFunc: func(obj interface{}) {
				dr, ok := obj.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				nlog.Debugf("Found DomainRoute(%s/%s) deleted", dr.Namespace, dr.Name)
			},
		},
	)

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *controller) Run(threadiness int) error {
	defer utilruntime.HandleCrash()
	defer c.domainRouteWorkqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	nlog.Info("Starting DomainRoute controller")
	c.kusciaInformerFactory.Start(c.ctx.Done())
	mrand.Seed(time.Now().UnixNano())

	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.gatewayListerSynced, c.domainRouteListerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			c.runWorker(c.ctx)
		}, time.Second, c.ctx.Done())
	}
	nlog.Info("Started DomainRoute controller")
	<-c.ctx.Done()
	return nil
}

func (c *controller) Name() string {
	return controllerName
}

// controller is the controller implementation for ClusterDomainRoute resources
func (c *controller) runWorker(ctx context.Context) {
	for queue.HandleQueueItemWithAlwaysRetry(ctx, controllerName, c.domainRouteWorkqueue, c.syncHandler) {
	}
}

func (c *controller) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the DomainRoute resource with this namespace/name
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Errorf("clusterdomainroute '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}

	if ensureLabels(ctx, c.kusciaClient, dr) {
		return nil
	}

	if err := DoValidate(&dr.Spec); err != nil {
		nlog.Error(err.Error())
		return nil
	}

	if dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken || dr.Spec.BodyEncryption != nil {
		if dr.Spec.TokenConfig == nil {
			return fmt.Errorf("tokenconfig cant be null")
		}
		if dr.Spec.TokenConfig.SourcePublicKey == "" || dr.Spec.TokenConfig.DestinationPublicKey == "" {
			nlog.Warnf("domainroute %s/%s select initializer failed, because source or destination pub is null, wait domain update", dr.Namespace, dr.Name)
			return nil
		}
		if namespace == dr.Spec.Source {
			if c.needRollingToNext(ctx, dr) {
				return c.preRollingSourceDomainRoute(ctx, dr)
			} else {
				if hasUpdate, err := c.ensureInitializer(ctx, dr); err != nil || hasUpdate {
					return err
				}
			}

			if dr.Status.TokenStatus.RevisionToken.IsReady {
				c.domainRouteWorkqueue.AddAfter(key, time.Until(dr.Status.TokenStatus.RevisionToken.ExpirationTime.Time))
				if dr.Spec.TokenConfig.RollingUpdatePeriod > 0 {
					c.domainRouteWorkqueue.AddAfter(key, time.Until(dr.Status.TokenStatus.RevisionToken.RevisionTime.Time.Add(time.Second*time.Duration(dr.Spec.TokenConfig.RollingUpdatePeriod))))
				}
				return c.postRollingSourceDomainRoute(ctx, dr)
			}
		} else if namespace == dr.Spec.Destination {
			if dr.Spec.TokenConfig.RollingUpdatePeriod > 0 {
				if len(dr.Status.TokenStatus.Tokens) > 0 && time.Since(dr.Status.TokenStatus.Tokens[0].ExpirationTime.Time) > domainRouteSyncPeriod {
					dr = dr.DeepCopy()
					rev := dr.Status.TokenStatus.Tokens[0].Revision
					dr.Status.TokenStatus.Tokens = dr.Status.TokenStatus.Tokens[1:]
					_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(c.ctx, dr, metav1.UpdateOptions{})
					if err == nil {
						nlog.Infof("domainroute %s/%s delete expirated token %d", dr.Namespace, dr.Name, rev)
					}
					return err
				}
			}
			c.postRollingDestinationDomainRoute(ctx, dr)
		}
	}
	return nil
}

func (c *controller) syncDomainRouteByNamespace(namespace string) {
	drs, err := c.domainRouteLister.DomainRoutes(namespace).List(labels.Everything())
	if err != nil {
		nlog.Error(err)
		return
	}

	for _, dr := range drs {
		c.enqueueDomainRoute(dr)
	}
}

func ensureLabels(ctx context.Context, kusciaClient kusciaclientset.Interface, dr *kusciaapisv1alpha1.DomainRoute) bool {
	var err error
	if dr.Labels == nil {
		drCopy := dr.DeepCopy()
		drCopy.Labels = map[string]string{
			common.KusciaSourceKey:      dr.Spec.Source,
			common.KusciaDestinationKey: dr.Spec.Destination,
		}
		if _, err = kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).Update(ctx, drCopy, metav1.UpdateOptions{}); err != nil && !k8serrors.IsConflict(err) {
			nlog.Warnf("Update domainroute %s/%s error:%s", dr.Namespace, dr.Name, err.Error())
		}
		if err == nil {
			nlog.Infof("domainroute %s/%s add labels", dr.Namespace, dr.Name)
		}
		return true
	} else if _, ok := dr.Labels[common.KusciaSourceKey]; !ok {
		drCopy := dr.DeepCopy()
		drCopy.Labels[common.KusciaSourceKey] = dr.Spec.Source
		drCopy.Labels[common.KusciaDestinationKey] = dr.Spec.Destination
		if _, err = kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).Update(ctx, drCopy, metav1.UpdateOptions{}); err != nil && !k8serrors.IsConflict(err) {
			nlog.Warnf("Update domainroute %s/%s error:%s", dr.Namespace, dr.Name, err.Error())
		}
		if err == nil {
			nlog.Infof("domainroute %s/%s add labels", dr.Namespace, dr.Name)
		}
		return true
	}
	return false
}

func (c *controller) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *controller) enqueueDomainRoute(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.domainRouteWorkqueue)
}
