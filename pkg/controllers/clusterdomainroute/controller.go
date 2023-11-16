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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
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
	syncDomainPubKeyReason       = "SyncDomainPubKey"
	errErrResourceExists         = "ErrResourceExists"
	doValidateReason             = "DoValidate"
	checkDomainRoute             = "CheckDomainRoute"
	syncDomainRouteStatus        = "SyncDomainRouteStatus"
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

				nlog.Debugf("Found clusterdomain(%s) update", newCdr.Name)
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

func addLabel(ctx context.Context, kusciaClient kusciaclientset.Interface, cdr *kusciaapisv1alpha1.ClusterDomainRoute) *kusciaapisv1alpha1.ClusterDomainRoute {
	var err error
	if cdr.Labels == nil {
		cdrCopy := cdr.DeepCopy()
		cdrCopy.Labels = map[string]string{
			common.KusciaSourceKey:      cdr.Spec.Source,
			common.KusciaDestinationKey: cdr.Spec.Destination,
		}
		if cdr, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{}); err != nil {
			nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdr.Spec.Source, cdr.Spec.Destination, err.Error())
		}
	} else if _, ok := cdr.Labels[common.KusciaSourceKey]; !ok {
		cdrCopy := cdr.DeepCopy()
		cdrCopy.Labels[common.KusciaSourceKey] = cdr.Spec.Source
		cdrCopy.Labels[common.KusciaDestinationKey] = cdr.Spec.Destination
		if cdr, err = kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{}); err != nil {
			nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdr.Spec.Source, cdr.Spec.Destination, err.Error())
		}
	}
	return cdr
}

func (c *controller) checkDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute,
	role kusciaapisv1alpha1.DomainRole, namespace, drName string) (*kusciaapisv1alpha1.DomainRoute, error) {
	if role == kusciaapisv1alpha1.Partner {
		return nil, nil
	}
	if cdr.Spec.Destination == c.Namespace && namespace != c.Namespace {
		return nil, nil
	}
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(drName)
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Not found domainroute %s/%s, so create it", namespace, drName)
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
	if needDeleteDr(cdr, dr) {
		return nil, c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Delete(ctx, dr.Name, metav1.DeleteOptions{})
	}

	if !compareSpec(cdr, dr) {
		dr = dr.DeepCopy()
		dr.Labels = cdr.Labels
		dr.Spec = cdr.Spec.DomainRouteSpec
		nlog.Infof("Found domainroute %s/%s not match cdr, correct it", namespace, drName)
		if dr, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Update(ctx, dr, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
	}
	return dr, nil
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

	cdr, err = c.syncDomainPubKey(ctx, cdr)
	if err != nil {
		nlog.Errorf("clusterdomainroute %s SyncDomainPubKey error:%v", cdr.Name, err)
		return err
	}

	if err := domainroute.DoValidate(&cdr.Spec.DomainRouteSpec); err != nil {
		nlog.Errorf("clusterdomainroute %s doValidate error:%v", cdr.Name, err)
		return err
	}

	sourceRole, destRole, err := c.getDomainRole(cdr)
	if err != nil {
		return err
	}

	drName := fmt.Sprintf("%s-%s", cdr.Spec.Source, cdr.Spec.Destination)

	// Create domainroute in source namespace
	srcdr, err := c.checkDomainRoute(ctx, cdr, sourceRole, cdr.Spec.Source, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}

	// Create domainroute in destination namespace
	destdr, err := c.checkDomainRoute(ctx, cdr, destRole, cdr.Spec.Destination, drName)
	if err != nil {
		nlog.Error(err.Error())
		return err
	}

	appendLabels := make(map[string]string, 0)
	if sourceRole == kusciaapisv1alpha1.Partner {
		appendLabels[common.LabelDomainRoutePartner] = cdr.Spec.Source
	} else if destRole == kusciaapisv1alpha1.Partner {
		appendLabels[common.LabelDomainRoutePartner] = cdr.Spec.Destination
	}

	cdr, err = c.updateLabel(ctx, cdr, appendLabels, nil)
	if err != nil {
		return err
	}

	cdr, err = c.syncStatusFromDomainroute(cdr, srcdr, destdr)
	if err != nil {
		return err
	}

	return c.checkInteropConfig(ctx, cdr, sourceRole, destRole)
}

func (c *controller) enqueueClusterDomainRoute(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.clusterDomainRouteWorkqueue)
}

func (c *controller) syncDomainPubKey(ctx context.Context,
	cdr *kusciaapisv1alpha1.ClusterDomainRoute) (*kusciaapisv1alpha1.ClusterDomainRoute, error) {
	if cdr.Spec.TokenConfig != nil && cdr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA || cdr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenUIDRSA {
		cdrCopy := cdr.DeepCopy()
		needUpdate := false

		srcRsaPubData := c.getPublicKeyFromDomain(cdr.Spec.Source)
		srcRsaPub := base64.StdEncoding.EncodeToString(srcRsaPubData)
		if len(srcRsaPubData) != 0 && cdr.Spec.TokenConfig.SourcePublicKey != srcRsaPub {
			cdrCopy.Spec.TokenConfig.SourcePublicKey = srcRsaPub
			needUpdate = true
		}

		destRsaPubData := c.getPublicKeyFromDomain(cdr.Spec.Destination)
		destRsaPub := base64.StdEncoding.EncodeToString(destRsaPubData)
		if len(destRsaPubData) != 0 && cdr.Spec.TokenConfig.DestinationPublicKey != destRsaPub {
			cdrCopy.Spec.TokenConfig.DestinationPublicKey = destRsaPub
			needUpdate = true
		}

		if needUpdate {
			cdr, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy,
				metav1.UpdateOptions{})
			return cdr, err
		}
	}
	return cdr, nil
}

func (c *controller) syncClusterDomainRouteByDomainName(key, domainName string) {
	cdrReqSrc, _ := labels.NewRequirement(key, selection.Equals, []string{domainName})
	cdrs, err := c.clusterDomainRouteLister.List(labels.NewSelector().Add(*cdrReqSrc))
	if err != nil {
		nlog.Error(err)
		return
	}

	for _, cdr := range cdrs {
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
	}, time.Minute, c.ctx.Done())
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

func compareSpec(cdr *kusciaapisv1alpha1.ClusterDomainRoute, dr *kusciaapisv1alpha1.DomainRoute) bool {
	if !reflect.DeepEqual(cdr.Labels, dr.Labels) {
		return false
	}

	if !reflect.DeepEqual(cdr.Spec.DomainRouteSpec, dr.Spec) {
		return false
	}

	return true
}

func (c *controller) getPublicKeyFromDomain(namespace string) []byte {
	domain, err := c.domainLister.Get(namespace)
	if err != nil {
		return nil
	}
	if domain.Spec.Cert != "" {
		rsaPubData, err := getPublickeyFromCert(domain.Spec.Cert)
		if err != nil {
			return nil
		}
		return rsaPubData
	}
	return nil
}

func (c *controller) Name() string {
	return controllerName
}

func (c *controller) syncStatusFromDomainroute(cdr *kusciaapisv1alpha1.ClusterDomainRoute,
	srcdr *kusciaapisv1alpha1.DomainRoute, destdr *kusciaapisv1alpha1.DomainRoute) (*kusciaapisv1alpha1.ClusterDomainRoute, error) {
	needUpdate := false
	cdr = cdr.DeepCopy()
	setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionTrue, "", "Success"))
	if srcdr != nil && !reflect.DeepEqual(cdr.Status.TokenStatus.SourceTokens, srcdr.Status.TokenStatus.Tokens) {
		cdr.Status.TokenStatus.SourceTokens = srcdr.Status.TokenStatus.Tokens
		needUpdate = true
		if len(cdr.Status.TokenStatus.SourceTokens) == 0 {
			if !srcdr.Status.IsDestinationAuthrized {
				setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "DestinationIsNotAuthrized", "TokenNotGenerate"))
			} else {
				setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenNotGenerate", "TokenNotGenerate"))
			}
		}
	}
	if destdr != nil && !reflect.DeepEqual(cdr.Status.TokenStatus.DestinationTokens, destdr.Status.TokenStatus.Tokens) {
		cdr.Status.TokenStatus.DestinationTokens = destdr.Status.TokenStatus.Tokens
		needUpdate = true
		if len(cdr.Status.TokenStatus.DestinationTokens) == 0 {
			setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenNotGenerate", "TokenNotGenerate"))
		}
	}

	if needUpdate {
		sn := len(cdr.Status.TokenStatus.SourceTokens)
		dn := len(cdr.Status.TokenStatus.DestinationTokens)
		if sn > 0 && dn > 0 && cdr.Status.TokenStatus.SourceTokens[sn-1].Revision != cdr.Status.TokenStatus.DestinationTokens[dn-1].Revision {
			setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenRevisionNotMatch", "TokenRevisionNotMatch"))
		}

		cdr, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(c.ctx, cdr, metav1.UpdateOptions{})
		return cdr, err
	}

	return cdr, nil
}

func (c *controller) getDomainRole(cdr *kusciaapisv1alpha1.ClusterDomainRoute) (kusciaapisv1alpha1.DomainRole,
	kusciaapisv1alpha1.DomainRole, error) {
	s, err := c.domainLister.Get(cdr.Spec.Source)
	if err != nil {
		nlog.Warnf("get Domain %s fail: %v", cdr.Spec.Source, err)
		return "", "", err
	}

	d, err := c.domainLister.Get(cdr.Spec.Destination)
	if err != nil {
		nlog.Warnf("get Domain %s fail: %v", cdr.Spec.Destination, err)
		return "", "", err
	}

	return s.Spec.Role, d.Spec.Role, nil
}

func (c *controller) updateLabel(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, addLabels map[string]string,
	removeLabels map[string]struct{}) (*kusciaapisv1alpha1.ClusterDomainRoute, error) {
	var err error

	needUpdateLabel := func() bool {
		if cdr.Labels == nil {
			return true
		}

		_, ok := cdr.Labels[common.KusciaSourceKey]
		if !ok {
			return true
		}

		for k, v := range addLabels {
			if oldVal, exist := cdr.Labels[k]; !exist || oldVal != v {
				return true
			}
		}

		for k, _ := range removeLabels {
			if _, exist := cdr.Labels[k]; exist {
				return true
			}
		}

		return false
	}

	if !needUpdateLabel() {
		return cdr, nil
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

	if cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{}); err != nil {
		nlog.Errorf("Update cdr, src(%s) dst(%s) failed with (%s)", cdrCopy.Spec.Source, cdrCopy.Spec.Destination, err.Error())
		return cdr, err
	}

	return cdr, nil
}

func newCondition(condType kusciaapisv1alpha1.ClusterDomainRouteConditionType, status corev1.ConditionStatus, reason, message string) *kusciaapisv1alpha1.ClusterDomainRouteCondition {
	return &kusciaapisv1alpha1.ClusterDomainRouteCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func setCondition(status *kusciaapisv1alpha1.ClusterDomainRouteStatus, condition *kusciaapisv1alpha1.ClusterDomainRouteCondition) {
	var currentCond *kusciaapisv1alpha1.ClusterDomainRouteCondition
	for i := range status.Conditions {
		cond := &status.Conditions[i]

		if cond.Type != condition.Type {
			// DO NOT TOUCH READY CONDITION
			if cond.Type == kusciaapisv1alpha1.ClusterDomainRouteReady || condition.Type == kusciaapisv1alpha1.ClusterDomainRouteReady {
				continue
			}

			if cond.Status == corev1.ConditionTrue {
				cond.Status = corev1.ConditionFalse
				cond.LastUpdateTime = condition.LastTransitionTime
				cond.LastTransitionTime = condition.LastTransitionTime
				cond.Reason = condition.Reason
				cond.Message = condition.Message
			}
			continue
		}

		currentCond = cond
		// Do not update lastTransitionTime if the status of the condition doesn't change.
		if cond.Status == condition.Status {
			condition.LastTransitionTime = cond.LastTransitionTime
		}
		status.Conditions[i] = *condition
	}

	if currentCond == nil {
		status.Conditions = append(status.Conditions, *condition)
	}
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
