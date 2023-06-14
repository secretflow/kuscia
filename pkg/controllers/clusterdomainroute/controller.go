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
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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
func NewController(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	kusciaClient kusciaclientset.Interface,
	eventRecorder record.EventRecorder,
) controllers.IController {
	kusciaInformerFactory := informers.NewSharedInformerFactory(kusciaClient, time.Second*30)
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
	// Set up an event handler for when Namespace resources change
	domainInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldone, newone interface{}) {
				controller.syncClusterDomainRoute(oldone, newone)
			},
		},
	)

	// Set up an event handler for when clusterdomainroute resources change
	clusterDomainRouteInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: controller.enqueueClusterDomainRoute,
			UpdateFunc: func(_, newone interface{}) {
				controller.enqueueClusterDomainRoute(newone)
			},
			DeleteFunc: controller.deleteClusterDomainRoute,
		},
		clusterDomainRouteSyncPeriod,
	)

	// Set up an event handler for when DomainRoute resources change
	domainRouteInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: controller.handleObject,
			UpdateFunc: func(oldone, newone interface{}) {
				newGwr, ok := newone.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				oldGwr, ok := oldone.(*kusciaapisv1alpha1.DomainRoute)
				if !ok {
					return
				}
				if newGwr.ResourceVersion == oldGwr.ResourceVersion {
					// Periodic resync will send update events for all known DomainRoutes.
					// Two different versions of the same DomainRoute will always have different RVs.
					return
				}
				controller.handleObject(newone)
			},
			DeleteFunc: controller.handleObject,
		},
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
	go wait.Until(c.Monitorcdrstatus, time.Minute*3, c.ctx.Done())

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
					return fmt.Errorf("SourcePublicKey[%s] is format err, must be base64 encoded, err :%v ", cdr.Name, err)
				}
				if _, err := base64.StdEncoding.DecodeString(cdr.Spec.TokenConfig.DestinationPublicKey); err != nil {
					return fmt.Errorf("DestinationPublicKey[%s] is format err, must be base64 encoded, err :%v ", cdr.Name, err)
				}
				return nil
			}
			// If not be set, sync public key from domain
			if err := c.syncDomainPubKey(ctx, cdr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Controller) createDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, namespace, drName string) (*kusciaapisv1alpha1.DomainRoute, bool, error) {
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(drName)
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Create domainroute %s/%s", namespace, drName)
		dr, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Create(ctx, newDomainRoute(cdr, drName, namespace), metav1.CreateOptions{})
	}

	if err != nil {
		return nil, false, err
	}

	if !metav1.IsControlledBy(dr, cdr) {
		msg := fmt.Sprintf("DomainRoute %s already exists in namespace %s and is not managed by ClusterDomainRoute", drName, namespace)
		c.recorder.Event(cdr, corev1.EventTypeWarning, errErrResourceExists, msg)
		return nil, false, fmt.Errorf(msg)
	}

	if !compareSpec(cdr, dr) {
		nlog.Infof("Delete domainroute %s/%s", namespace, drName)
		if err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Delete(ctx, drName, metav1.DeleteOptions{}); err != nil {
			return nil, false, err
		}
		return dr, true, nil
	}
	return dr, false, nil
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

	if err = c.doValidate(ctx, cdr); err != nil {
		nlog.Error(err.Error())
		return err
	}

	drName := fmt.Sprintf("%s-%s", cdr.Spec.Source, cdr.Spec.Destination)

	// Create domainroute in source namespace
	srcDr, srcChanged, err := c.createDomainRoute(ctx, cdr, cdr.Spec.Source, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}
	// Create domainroute in destination namespace
	destDr, destChanged, err := c.createDomainRoute(ctx, cdr, cdr.Spec.Destination, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}

	// Force rolling update if specification has changed.
	if srcChanged || destChanged {
		c.preRollingClusterDomainRoute(ctx, cdr, true)
		return fmt.Errorf("sourece or destination is changed, will retry")
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

func (c *Controller) syncDomainPubKey(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) error {
	srcRsaPubData, err := c.getPublicKeyFromDomain(ctx, cdr.Name, cdr.Spec.Source)
	if err != nil {
		return err
	}
	destRsaPubData, err := c.getPublicKeyFromDomain(ctx, cdr.Name, cdr.Spec.Destination)
	if err != nil {
		return err
	}

	nlog.Infof("Sync domain %s cert with clusterdomainroute", cdr.Name)

	cdrCopy := cdr.DeepCopy()
	cdrCopy.Spec.TokenConfig.SourcePublicKey = base64.StdEncoding.EncodeToString(srcRsaPubData)
	cdrCopy.Spec.TokenConfig.DestinationPublicKey = base64.StdEncoding.EncodeToString(destRsaPubData)

	c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{})
	return fmt.Errorf("sync domain %s cert with clusterdomainroute, will retry", cdr.Name)
}

func (c *Controller) syncClusterDomainRoute(oldOne, newOne interface{}) {
	oldNS, ok := oldOne.(*kusciaapisv1alpha1.Domain)
	if !ok {
		return
	}
	newNS, ok := newOne.(*kusciaapisv1alpha1.Domain)
	if !ok {
		return
	}
	// If public key doesn't change, do nothing.
	if newNS.Spec.Cert == oldNS.Spec.Cert {
		return
	}

	cdrs, err := c.clusterDomainRouteLister.List(labels.Everything())
	if err != nil {
		nlog.Error(err)
		return
	}

	nlog.Infof("Syncing domain %s public key", newNS.Name)
	for _, cdr := range cdrs {
		if cdr.Spec.Source == newNS.Name || cdr.Spec.Destination == newNS.Name {
			c.workqueue.Add(cdr.Name)
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

// handleObject will take any resource implementing metav1.Object and attempt
// to find the ClusterDomainRoute resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that ClusterDomainRoute resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Error("error decoding object, invalid type")
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			nlog.Error("error decoding object tombstone, invalid type")
			return
		}
		nlog.Debugf("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	nlog.Debugf("Processing object: %s, type:%s", object.GetName(), reflect.TypeOf(obj))
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a ClusterDomainRoute, we should not do anything more with it.
		if ownerRef.Kind != "ClusterDomainRoute" {
			nlog.Warnf("kind is not ClusterDomainRoute, is %s", ownerRef.Kind)
			return
		}

		cdr, err := c.clusterDomainRouteLister.Get(ownerRef.Name)
		if err != nil {
			nlog.Debugf("Ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueClusterDomainRoute(cdr)
		return
	}
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
	rollingUpdatePeriod := time.Duration(cdr.Spec.TokenConfig.RollingUpdatePeriod) * time.Second
	if cdr.Status.TokenStatus.Revision == 0 {
		c.preRollingClusterDomainRoute(ctx, cdr, false)
		return fmt.Errorf("clusterdomainroute %s token revision is 0, so prerolling clusterdomainroute and will retry", cdr.Name)
	}
	if rollingUpdatePeriod > 0 && time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > rollingUpdatePeriod {
		oldtime := cdr.Status.TokenStatus.RevisionTime.Time
		c.preRollingClusterDomainRoute(ctx, cdr, false)
		return fmt.Errorf("clusterdomainroute %s token is out of time(rollingUpdatePeriod %s, rollingUpdatePeriod %ds), so prerolling clusterdomainroute and will retry", cdr.Name, oldtime.String(), cdr.Spec.TokenConfig.RollingUpdatePeriod)
	}

	if cdr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken || cdr.Spec.BodyEncryption != nil {
		if cdr.Status.TokenStatus.Revision != srcdr.Status.TokenStatus.RevisionToken.Revision || cdr.Status.TokenStatus.Revision != destdr.Status.TokenStatus.RevisionToken.Revision {
			// rolling to next revision to invalidate flying token negotiation
			if time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > clusterDomainRouteSyncPeriod {
				c.preRollingClusterDomainRoute(ctx, cdr, false)
				return fmt.Errorf("clusterdomainroute %s token is out of clusterDomainRouteSyncPeriod, so prerolling clusterdomainroute and will retry", cdr.Name)
			}

			switch cdr.Spec.TokenConfig.TokenGenMethod {
			case kusciaapisv1alpha1.TokenGenMethodRAND:
				c.intraRollingClusterDomainRouteRand(ctx, cdr, srcdr, destdr)
				return fmt.Errorf("%s intraRollingClusterDomainRouteRand and will retry", cdr.Name)
			case kusciaapisv1alpha1.TokenGenMethodRSA:
				c.intraRollingClusterDomainRouteRSA(ctx, cdr, srcdr, destdr)
				return fmt.Errorf("%s intraRollingClusterDomainRouteRSA and will retry", cdr.Name)
			default:
				nlog.Warnf("Clusterdomainroute %s unexpected token generation method: %s", cdr.Name, cdr.Spec.TokenConfig.TokenGenMethod)
				return nil
			}
		}

		if srcdr.Status.TokenStatus.RevisionToken.Token == "" || destdr.Status.TokenStatus.RevisionToken.Token == "" {
			// rolling to next revision to invalidate flying token negotiation
			if time.Since(cdr.Status.TokenStatus.RevisionTime.Time) > clusterDomainRouteSyncPeriod {
				c.preRollingClusterDomainRoute(ctx, cdr, false)
				return fmt.Errorf("clusterdomainroute %s token is out of clusterDomainRouteSyncPeriod, so prerolling clusterdomainroute and will retry", cdr.Name)
			}

			return fmt.Errorf("clusterdomainroute %s is syncing, retry to wait", cdr.Name)
		}
		return c.postRollingClusterDomainRoute(ctx, cdr, srcdr, destdr)
	}
	nlog.Debugf("clusterdomainroute %s has nothing to do", cdr.Name)
	return nil
}

func (c *Controller) Name() string {
	return controllerName
}

func CheckCRDExists(ctx context.Context, extensionClient apiextensionsclientset.Interface) error {
	if _, err := extensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, controllers.CRDDomainsName, metav1.GetOptions{}); err != nil {
		return err
	}
	if _, err := extensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, controllers.CRDClusterDomainRoutesName, metav1.GetOptions{}); err != nil {
		return err
	}
	if _, err := extensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, controllers.CRDDomainRoutesName, metav1.GetOptions{}); err != nil {
		return err
	}
	if _, err := extensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, controllers.CRDGatewaysName, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}
