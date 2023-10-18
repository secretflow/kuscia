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

package clusterdomainroute

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	mrand "math/rand"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
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
	clusterDomainRouteSyncPeriod = 2 * time.Minute
	gatewayLiveTimeout           = 3 * time.Minute
	controllerName               = "domain-route-controller"
	errErrResourceExists         = "ErrResourceExists"
)

// Controller is the controller implementation for ClusterDomainRoute resources
type Controller struct {
	cancel context.CancelFunc
	ctx    context.Context

	clusterDomainRouteLister       kuscialistersv1alpha1.ClusterDomainRouteLister
	clusterDomainRouteListerSynced cache.InformerSynced
	domainLister                   kuscialistersv1alpha1.DomainLister
	domainListerSynced             cache.InformerSynced
	domainRouteLister              kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced        cache.InformerSynced
	gatewayLister                  kuscialistersv1alpha1.GatewayLister
	gatewayListerSynced            cache.InformerSynced

	kubeClient            kubernetes.Interface
	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory informers.SharedInformerFactory

	recorder  record.EventRecorder
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a new sample controller
func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	eventRecorder := config.EventRecorder
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, clusterDomainRouteSyncPeriod)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	gatewayInformer := kusciaInformerFactory.Kuscia().V1alpha1().Gateways()
	clusterDomainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().ClusterDomainRoutes()
	domainRouteInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()
	controller := &Controller{
		kubeClient:                     kubeClient,
		kusciaClient:                   kusciaClient,
		kusciaInformerFactory:          kusciaInformerFactory,
		domainLister:                   domainInformer.Lister(),
		domainListerSynced:             domainInformer.Informer().HasSynced,
		gatewayLister:                  gatewayInformer.Lister(),
		gatewayListerSynced:            gatewayInformer.Informer().HasSynced,
		clusterDomainRouteLister:       clusterDomainRouteInformer.Lister(),
		clusterDomainRouteListerSynced: clusterDomainRouteInformer.Informer().HasSynced,
		domainRouteLister:              domainRouteInformer.Lister(),
		domainRouteListerSynced:        domainRouteInformer.Informer().HasSynced,
		workqueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ClusterDomainRoutes"),
		recorder:                       eventRecorder,
	}
	controller.ctx, controller.cancel = context.WithCancel(ctx)

	nlog.Info("Setting up event handlers")
	gatewayInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(newOne interface{}) {
				newGateway, ok := newOne.(*kusciaapisv1alpha1.Gateway)
				if !ok {
					return
				}
				nlog.Infof("Syncing ClusterDomainRoute because found new Gateway(%s)", newGateway.Namespace)
				controller.syncClusterDomainRoute(newGateway.Namespace)
			},
		},
		0,
	)

	// Set up an event handler for when Namespace resources change
	domainInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(newOne interface{}) {
				newDomain, ok := newOne.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				nlog.Infof("Syncing ClusterDomainRoute because found new Domain(%s)", newDomain.Name)
				controller.syncClusterDomainRoute(newDomain.Name)
			},
			UpdateFunc: func(oldOne, newOne interface{}) {
				oldDomain, ok := oldOne.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				newDomain, ok := newOne.(*kusciaapisv1alpha1.Domain)
				if !ok {
					return
				}
				// If public key doesn't change, do nothing.
				if newDomain.Spec.Cert == oldDomain.Spec.Cert {
					return
				}
				nlog.Infof("Syncing ClusterDomainRoute because Domain(%s) cert is changed", newDomain.Name)
				controller.syncClusterDomainRoute(newDomain.Name)
			},
		},
		0, // no resync, just wait event or wait Clusterdomainroute's resync.
	)

	// Set up an event handler for when clusterdomainroute resources change
	clusterDomainRouteInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: controller.enqueueClusterDomainRoute,
			UpdateFunc: func(_, newone interface{}) {
				controller.enqueueClusterDomainRoute(newone)
			},
			DeleteFunc: controller.deleteClusterDomainRoute,
		},
	)

	// Set up an event handler for when DomainRoute resources change
	domainRouteInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			// AddFunc is unnecessary. Clusterdomainroute's handler will create it and continue to process.
			UpdateFunc: func(oldone, newone interface{}) {
				newDr, ok := newone.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				cdr, err := controller.clusterDomainRouteLister.Get(newDr.Name)
				if err != nil {
					nlog.Warnf("Get ClusterDomainRoute by DomainRoute(%s/%s) fail", newDr.Namespace, newDr.Name)
					return
				}
				nlog.Infof("Syncing ClusterDomainRoute because found DomainRoute(%s/%s) update", newDr.Namespace, newDr.Name)
				controller.enqueueClusterDomainRoute(cdr)
			},
			DeleteFunc: func(obj interface{}) {
				oldDr, ok := obj.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				cdr, err := controller.clusterDomainRouteLister.Get(oldDr.Name)
				if err != nil {
					nlog.Warnf("Get ClusterDomainRoute by DomainRoute(%s/%s) fail", oldDr.Namespace, oldDr.Name)
					return
				}
				nlog.Infof("Syncing ClusterDomainRoute because found DomainRoute(%s/%s) deleted", oldDr.Namespace, oldDr.Name)
				controller.enqueueClusterDomainRoute(cdr)
			},
		},
		0,
	)

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	nlog.Info("Starting ClusterDomainRoute controller")
	c.kusciaInformerFactory.Start(c.ctx.Done())
	// Wait for the caches to be synced before starting workers
	nlog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.domainListerSynced, c.gatewayListerSynced, c.clusterDomainRouteListerSynced, c.domainRouteListerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	mrand.Seed(time.Now().UnixNano())
	go wait.Until(c.Monitorcdrstatus, time.Minute, c.ctx.Done())

	nlog.Info("Starting workers")
	// Launch two workers to process ClusterDomainRoute resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, c.ctx.Done())
	}

	nlog.Info("Started workers")
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
	c.workqueue.ShutDown()
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for queue.HandleQueueItemWithAlwaysRetry(context.Background(), controllerName, c.workqueue, c.syncHandler) {
	}
}

func (c *Controller) addLabel(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) *kusciaapisv1alpha1.ClusterDomainRoute {
	var err error
	// add labels
	if cdr.Labels == nil {
		cdrCopy := cdr.DeepCopy()
		cdrCopy.Labels = map[string]string{
			common.KusciaSourceKey:      cdr.Spec.Source,
			common.KusciaDestinationKey: cdr.Spec.Destination,
		}
		if cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{}); err != nil {
			nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdr.Spec.Source, cdr.Spec.Destination, err.Error())
		}
	} else if _, ok := cdr.Labels[common.KusciaSourceKey]; !ok {
		cdrCopy := cdr.DeepCopy()
		cdrCopy.Labels[common.KusciaSourceKey] = cdr.Spec.Source
		cdrCopy.Labels[common.KusciaDestinationKey] = cdr.Spec.Destination
		if cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{}); err != nil {
			nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdr.Spec.Source, cdr.Spec.Destination, err.Error())
		}
	}
	return cdr
}

func (c *Controller) doValidate(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) error {
	if cdr.Spec.Source == "" || cdr.Spec.Destination == "" {
		return fmt.Errorf("%s source or destination is null", cdr.Name)
	}
	// body encryption will use tokenConfig to generate encrypt key
	if cdr.Spec.BodyEncryption != nil {
		if cdr.Spec.TokenConfig == nil {
			return fmt.Errorf("clusterdomainroute %s Spec.Authentication.Token is null", cdr.Name)
		}
	}

	switch cdr.Spec.AuthenticationType {
	case kusciaapisv1alpha1.DomainAuthenticationToken:
		if cdr.Spec.TokenConfig == nil {
			return fmt.Errorf("clusterdomainroute %s Spec.Authentication.Token is null", cdr.Name)
		}
	case kusciaapisv1alpha1.DomainAuthenticationMTLS:
		if cdr.Spec.MTLSConfig == nil {
			return fmt.Errorf("clusterdomainroute %s Spec.MTLSConfig is null", cdr.Name)
		}
		if cdr.Spec.MTLSConfig.SourceClientCert == "" {
			return fmt.Errorf("clusterdomainroute %s Spec.MTLSConfig.SourceClientCert is null", cdr.Name)
		}
		if _, err := base64.StdEncoding.DecodeString(cdr.Spec.MTLSConfig.SourceClientCert); err != nil {
			return fmt.Errorf("clusterdomainroute %s Spec.MTLSConfig.SourceClientCert is format error, must be base64 encoded", cdr.Name)
		}
		if cdr.Spec.MTLSConfig.TLSCA != "" {
			if _, err := base64.StdEncoding.DecodeString(cdr.Spec.MTLSConfig.TLSCA); err != nil {
				return fmt.Errorf("clusterdomainroute %s Spec.MTLSConfig.TLSCA is format error, must be base64 encoded", cdr.Name)
			}
		}
		if cdr.Spec.MTLSConfig.SourceClientPrivateKey != "" {
			if _, err := base64.StdEncoding.DecodeString(cdr.Spec.MTLSConfig.SourceClientPrivateKey); err != nil {
				return fmt.Errorf("clusterdomainroute %s Spec.MTLSConfig.SourceClientPrivateKey is format error, must be base64 encoded", cdr.Name)
			}
		}
	case kusciaapisv1alpha1.DomainAuthenticationNone:
	default:
		return fmt.Errorf("unsupport type %s", cdr.Spec.AuthenticationType)
	}

	if cdr.Spec.TokenConfig != nil {
		// Only check public key if TokenGenMethodRSA is RSA
		if cdr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA {
			if cdr.Spec.TokenConfig.SourcePublicKey != "" && cdr.Spec.TokenConfig.DestinationPublicKey != "" {
				// publickey must be base64 encoded
				if _, err := base64.StdEncoding.DecodeString(cdr.Spec.TokenConfig.SourcePublicKey); err != nil {
					return fmt.Errorf("sourcePublicKey[%s] is format err, must be base64 encoded, err :%v ", cdr.Name, err)
				}
				if _, err := base64.StdEncoding.DecodeString(cdr.Spec.TokenConfig.DestinationPublicKey); err != nil {
					return fmt.Errorf("destinationPublicKey[%s] is format err, must be base64 encoded, err :%v ", cdr.Name, err)
				}
			} else {
				// If not be set, sync public key from domain.
				c.syncDomainPubKey(ctx, cdr)
				return fmt.Errorf("%s's source or destination public key is nil, try to sync from domain cert and wait retry", cdr.Name)
			}
		}
	}
	return nil
}

func (c *Controller) checkDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, namespace, drName string) (*kusciaapisv1alpha1.DomainRoute, error) {
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(drName)
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Create domainroute %s/%s", namespace, drName)
		if dr, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Create(ctx, newDomainRoute(cdr, drName, namespace), metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if !metav1.IsControlledBy(dr, cdr) {
		msg := fmt.Sprintf("DomainRoute %s already exists in namespace %s and is not managed by ClusterDomainRoute", drName, namespace)
		return nil, fmt.Errorf("%s", msg)
	}

	if !compareSpec(cdr, dr) {
		nlog.Infof("Delete domainroute %s/%s", namespace, drName)
		if err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Delete(ctx, drName, metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return dr, nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ClusterDomainRoute resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
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

	cdr = c.addLabel(ctx, cdr)

	if err := c.doValidate(ctx, cdr); err != nil {
		nlog.Error(err.Error())
		return nil
	}

	drName := common.GenDomainRouteName(cdr.Spec.Source, cdr.Spec.Destination)

	// Create domainroute in source namespace
	srcDr, err := c.checkDomainRoute(ctx, cdr, cdr.Spec.Source, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}
	// Create domainroute in destination namespace
	destDr, err := c.checkDomainRoute(ctx, cdr, cdr.Spec.Destination, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}
	// Wait clusterdomainroute update event
	if srcDr == nil || destDr == nil {
		return nil
	}
	return c.updateClusterDomainRoute(ctx, cdr, srcDr, destDr)
}

func getPublickeyFromCert(certString string) ([]byte, error) {
	certPem, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return nil, err
	}
	certData, _ := pem.Decode(certPem)
	if certData == nil {
		return nil, fmt.Errorf("%s", "pem Decode fail")
	}
	cert, err := x509.ParseCertificate(certData.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s", "Cant get publickey from src domain")
	}
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(rsaPub),
	}
	return pem.EncodeToMemory(block), nil
}

func (c *Controller) getPublicKeyFromDomain(ctx context.Context, cdrName, namespace string) ([]byte, error) {
	domain, err := c.domainLister.Get(namespace)
	if err != nil {
		return nil, err
	}
	if domain.Spec.Cert != "" {
		rsaPubData, err := getPublickeyFromCert(domain.Spec.Cert)
		if err != nil {
			return nil, err
		}
		return rsaPubData, nil
	}
	return nil, fmt.Errorf("domain %s cert is nil", domain.Name)
}

func (c *Controller) syncDomainPubKey(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) {
	srcRsaPubData, err := c.getPublicKeyFromDomain(ctx, cdr.Name, cdr.Spec.Source)
	if err != nil {
		nlog.Warn(err)
		return
	}
	destRsaPubData, err := c.getPublicKeyFromDomain(ctx, cdr.Name, cdr.Spec.Destination)
	if err != nil {
		nlog.Warn(err)
		return
	}
	cdrCopy := cdr.DeepCopy()
	cdrCopy.Spec.TokenConfig.SourcePublicKey = base64.StdEncoding.EncodeToString(srcRsaPubData)
	cdrCopy.Spec.TokenConfig.DestinationPublicKey = base64.StdEncoding.EncodeToString(destRsaPubData)

	_, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{})
	if err != nil { // maybe network error, wait resync
		nlog.Warn(err)
		return
	}
}

func (c *Controller) syncClusterDomainRoute(domainName string) {
	cdrs, err := c.clusterDomainRouteLister.List(labels.Everything())
	if err != nil {
		nlog.Error(err)
		return
	}

	for _, cdr := range cdrs {
		if cdr.Spec.Source == domainName || cdr.Spec.Destination == domainName {
			c.enqueueClusterDomainRoute(cdr)
		}
	}
}

func (c *Controller) enqueueClusterDomainRoute(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.workqueue)
}

func (c *Controller) deleteClusterDomainRoute(obj interface{}) {
	cdr, ok := obj.(*kusciaapisv1alpha1.ClusterDomainRoute)
	if !ok {
		return
	}
	c.cleanMetrics(cdr.Name)
}

func newDomainRoute(cdr *kusciaapisv1alpha1.ClusterDomainRoute, name, namespace string) *kusciaapisv1alpha1.DomainRoute {
	return &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    cdr.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cdr, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("ClusterDomainRoute")),
			},
		},
		Spec:   cdr.Spec.DomainRouteSpec,
		Status: kusciaapisv1alpha1.DomainRouteStatus{},
	}
}

func (c *Controller) updateClusterDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, srcdr, destdr *kusciaapisv1alpha1.DomainRoute) error {
	if cdr.Spec.TokenConfig != nil {
		rollingUpdatePeriod := time.Duration(cdr.Spec.TokenConfig.RollingUpdatePeriod) * time.Second
		if cdr.Status.TokenStatus.Revision == 0 {
			c.preRollingClusterDomainRoute(ctx, cdr, false)
			return nil
		}
		if rollingUpdatePeriod > 0 && time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > rollingUpdatePeriod {
			c.preRollingClusterDomainRoute(ctx, cdr, false)
			nlog.Warnf("Clusterdomainroute %s token is out of time, rollingUpdate", cdr.Name)
			return nil
		}

		if cdr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken || cdr.Spec.BodyEncryption != nil {
			if cdr.Status.TokenStatus.Revision != srcdr.Status.TokenStatus.RevisionToken.Revision || cdr.Status.TokenStatus.Revision != destdr.Status.TokenStatus.RevisionToken.Revision {
				// rolling to next revision to invalidate flying token negotiation
				if time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > clusterDomainRouteSyncPeriod {
					c.preRollingClusterDomainRoute(ctx, cdr, false)
					nlog.Warnf("Clusterdomainroute %s token is out of SyncPeriod", cdr.Name)
					return nil
				}

				switch cdr.Spec.TokenConfig.TokenGenMethod {
				case kusciaapisv1alpha1.TokenGenMethodRAND:
					if err := c.intraRollingClusterDomainRouteRand(ctx, cdr, srcdr, destdr); err != nil {
						return err
					}
				case kusciaapisv1alpha1.TokenGenMethodRSA:
					if err := c.intraRollingClusterDomainRouteRSA(ctx, cdr, srcdr, destdr); err != nil {
						return err
					}
				default:
					nlog.Warnf("Clusterdomainroute %s unexpected token generation method: %s", cdr.Name, cdr.Spec.TokenConfig.TokenGenMethod)
					return nil
				}
			}

			if srcdr.Status.TokenStatus.RevisionToken.Token == "" || destdr.Status.TokenStatus.RevisionToken.Token == "" {
				// rolling to next revision to invalidate flying token negotiation
				if time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > clusterDomainRouteSyncPeriod {
					c.preRollingClusterDomainRoute(ctx, cdr, false)
					nlog.Warnf("Clusterdomainroute %s token is out of SyncPeriod", cdr.Name)
					return nil
				}

				return nil
			}
			return c.postRollingClusterDomainRoute(ctx, cdr, srcdr, destdr)
		}
	}
	nlog.Debugf("Clusterdomainroute %s has nothing to do", cdr.Name)
	return nil
}

func (c *Controller) Name() string {
	return controllerName
}
