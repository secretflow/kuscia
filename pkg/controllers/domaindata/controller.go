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

package domaindata

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"reflect"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	"github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	controllerName        = "domaindatagrant_controller"
	maxRetries            = 15
	getDomainCertErrorStr = "Get Domain cert error"
)

type Controller struct {
	ctx          context.Context
	cancel       context.CancelFunc
	Namespace    string
	RootDir      string
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface

	kusciaInformerFactory          kusciainformers.SharedInformerFactory
	domaindataLister               kuscialistersv1alpha1.DomainDataLister
	domaindatagrantLister          kuscialistersv1alpha1.DomainDataGrantLister
	domainLister                   kuscialistersv1alpha1.DomainLister
	domainDataGrantWorkqueue       workqueue.RateLimitingInterface
	domainDataGrantDeleteWorkqueue workqueue.RateLimitingInterface
	domainDataWorkqueue            workqueue.RateLimitingInterface
	domainDataDeleteWorkqueue      workqueue.RateLimitingInterface
	cacheSyncs                     []cache.InformerSynced
}

func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient

	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 1*time.Minute)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domaindataGrantInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()
	domaindataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	cacheSyncs := []cache.InformerSynced{
		domainInformer.Informer().HasSynced,
		domaindataGrantInformer.Informer().HasSynced,
		domaindataInformer.Informer().HasSynced,
	}
	controller := &Controller{
		Namespace:                      config.Namespace,
		RootDir:                        config.RootDir,
		kubeClient:                     kubeClient,
		kusciaClient:                   kusciaClient,
		kusciaInformerFactory:          kusciaInformerFactory,
		domaindatagrantLister:          domaindataGrantInformer.Lister(),
		domainLister:                   domainInformer.Lister(),
		domaindataLister:               domaindataInformer.Lister(),
		domainDataWorkqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "domaindata"),
		domainDataDeleteWorkqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "domaindatadelete"),
		domainDataGrantWorkqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "domaindatagrant"),
		domainDataGrantDeleteWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "domaindatagrantdelete"),
		cacheSyncs:                     cacheSyncs,
	}

	controller.ctx, controller.cancel = context.WithCancel(ctx)

	domaindataGrantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			queue.EnqueueObjectWithKey(obj, controller.domainDataGrantWorkqueue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			queue.EnqueueObjectWithKey(newObj, controller.domainDataGrantWorkqueue)
		},
		DeleteFunc: func(obj interface{}) {
			dd, ok := obj.(*v1alpha1.DomainDataGrant)
			if !ok {
				return
			}
			controller.domainDataGrantDeleteWorkqueue.Add(dd.Spec.Author + "/" + dd.Name)
		},
	})
	domaindataInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			queue.EnqueueObjectWithKey(newObj, controller.domainDataWorkqueue)
		},
		DeleteFunc: func(obj interface{}) {
			dd, ok := obj.(*v1alpha1.DomainData)
			if !ok {
				return
			}
			controller.domainDataDeleteWorkqueue.Add(dd.Spec.Author + "/" + dd.Name)
		},
	})
	return controller
}

func (c *Controller) deleteByOwnerDomainDataGrant(namespace, name string) error {
	r, err := labels.NewRequirement(common.LabelOwnerReferences, selection.Equals, []string{name})
	if err != nil {
		return err
	}
	ddgs, err := c.domaindatagrantLister.List(labels.NewSelector().Add(*r))
	if err != nil {
		return err
	}
	for _, ddg := range ddgs {
		err := c.kusciaClient.KusciaV1alpha1().DomainDataGrants(ddg.Namespace).Delete(c.ctx, ddg.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		r, _ = labels.NewRequirement(common.LabelDomainDataID, selection.Equals, []string{ddg.Spec.DomainDataID})
		ds, err := c.domaindatagrantLister.DomainDataGrants(ddg.Namespace).List(labels.NewSelector().Add(*r))
		if k8serrors.IsNotFound(err) || len(ds) == 0 {
			dd, err := c.domaindataLister.DomainDatas(ddg.Namespace).Get(ddg.Spec.DomainDataID)
			if err == nil {
				err := c.kusciaClient.KusciaV1alpha1().DomainDatas(dd.Namespace).Delete(c.ctx, dd.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Controller) deleteByOwnerDomainData(namespace, name string) error {
	r, _ := labels.NewRequirement(common.LabelOwnerReferences, selection.Equals, []string{name})
	ddgs, _ := c.domaindataLister.List(labels.NewSelector().Add(*r))
	for _, ddg := range ddgs {
		err := c.kusciaClient.KusciaV1alpha1().DomainDatas(ddg.Namespace).Delete(c.ctx, ddg.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int) error {
	defer runtime.HandleCrash()
	defer c.domainDataGrantWorkqueue.ShutDown()

	nlog.Info("Starting domain controller")
	c.kusciaInformerFactory.Start(c.ctx.Done())

	nlog.Info("Waiting for informer cache to sync")
	if !cache.WaitForCacheSync(c.ctx.Done(), c.cacheSyncs...) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runDomainDataGrantWorker, time.Second, c.ctx.Done())
		go wait.Until(c.runDomainDataWorker, time.Second, c.ctx.Done())
		go wait.Until(c.runDomainDataGrantDeleteWorker, time.Second, c.ctx.Done())
		go wait.Until(c.runDomainDataDeleteWorker, time.Second, c.ctx.Done())
	}

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

// runDomainDataGrantWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *Controller) runDomainDataGrantWorker() {
	for queue.HandleQueueItem(context.Background(), controllerName, c.domainDataGrantWorkqueue, c.syncDomainDataGrantHandler, maxRetries) {
	}
}

func (c *Controller) runDomainDataWorker() {
	for queue.HandleQueueItem(context.Background(), controllerName, c.domainDataWorkqueue, c.syncDomainDataHandler, maxRetries) {
	}
}

func (c *Controller) runDomainDataGrantDeleteWorker() {
	for queue.HandleQueueItem(context.Background(), controllerName, c.domainDataGrantDeleteWorkqueue, c.syncDomainDataGrantDeleteHandler, maxRetries) {
	}
}

func (c *Controller) runDomainDataDeleteWorker() {
	for queue.HandleQueueItem(context.Background(), controllerName, c.domainDataDeleteWorkqueue, c.syncDomainDataDeleteHandler, maxRetries) {
	}
}

// syncDomainDataGrantHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the domain resource
// with the current status of the resource.
func (c *Controller) syncDomainDataGrantHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("get DomainDataGrant(%s/%s) error", namespace, name)
		return nil
	}
	dg, err := c.domaindatagrantLister.DomainDataGrants(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Not found DomainDataGrant(%s/%s), may be deleted", namespace, name)
			return nil
		}
		return fmt.Errorf("get DomainDataGrant(%s/%s) error:%s", namespace, name, err.Error())
	}
	err = c.doValidate(dg)
	if err != nil {
		nlog.Warn(err)
		return nil
	}
	update, err := c.checkLabels(dg)
	if err != nil || update {
		return err
	}
	if dg.Spec.Author == namespace {
		domain, err := c.kusciaClient.KusciaV1alpha1().Domains().Get(c.ctx, dg.Spec.GrantDomain, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get domain %v err %v", dg.Spec.GrantDomain, err)
		}
		// 1. role is partner and master domain not empty => partner master domain
		// 2. role is partner and master domain empty => grant domain
		// 3. role is not partner => grant domain
		destDomain := dg.Spec.GrantDomain
		if domain.Spec.Role == v1alpha1.Partner && domain.Spec.MasterDomain != "" {
			destDomain = domain.Spec.MasterDomain
		}
		dgGrant, err := c.domaindatagrantLister.DomainDataGrants(destDomain).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = c.ensureDomainData(dg)
				if err != nil {
					return fmt.Errorf("ensureDomainData(%s/%s) error:%s", dg.Spec.GrantDomain, dg.Spec.DomainDataID, err.Error())
				}
				dgcopy := resources.ExtractDomainDataGrantSpec(dg)
				dgcopy.Namespace = destDomain
				dgcopy.Labels[common.LabelOwnerReferences] = dg.Name
				dgcopy.Labels[common.LabelDomainDataID] = dg.Spec.DomainDataID
				if domain.Spec.Role == v1alpha1.Partner {
					dgcopy.Annotations[common.InitiatorAnnotationKey] = dg.Spec.Author
					dgcopy.Annotations[common.InterConnKusciaPartyAnnotationKey] = dg.Spec.GrantDomain
				}
				_, err = c.kusciaClient.KusciaV1alpha1().DomainDataGrants(destDomain).Create(c.ctx, dgcopy, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("create DomainDataGrant(%s/%s) error:%s", dg.Spec.GrantDomain, name, err.Error())
				}
				nlog.Infof("Create DomainDataGrant %s success, because %s grant to %s", name, namespace, dg.Spec.GrantDomain)
			} else {
				return err
			}
		} else {
			update, err := c.ensureDomainDataGrantEqual(dg, dgGrant)
			if err != nil {
				return err
			}
			if update {
				return nil
			}
		}
	}
	return c.verify(dg)
}

func (c *Controller) syncDomainDataGrantDeleteHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("get DomainDataGrant(%s/%s) error", namespace, name)
		return nil
	}
	_, err = c.domaindatagrantLister.DomainDataGrants(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return c.deleteByOwnerDomainDataGrant(namespace, name)
		}
		return fmt.Errorf("get DomainDataGrant(%s/%s) error:%s", namespace, name, err.Error())
	}
	return nil
}

func (c *Controller) syncDomainDataDeleteHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("get DomainDataGrant(%s/%s) error", namespace, name)
		return nil
	}
	_, err = c.domaindataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return c.deleteByOwnerDomainData(namespace, name)
		}
		return fmt.Errorf("get DomainData(%s/%s) error:%s", namespace, name, err.Error())
	}

	return nil
}

func (c *Controller) ensureDomainDataGrantEqual(dg *v1alpha1.DomainDataGrant, dgGrant *v1alpha1.DomainDataGrant) (bool, error) {
	if !reflect.DeepEqual(dg.Spec, dgGrant.Spec) {
		dgGrant = dgGrant.DeepCopy()
		dgGrant.Spec = dg.Spec
		dgGrant.Labels[common.LabelOwnerReferences] = dg.Name
		_, err := c.kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Spec.GrantDomain).Update(c.ctx, dgGrant, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("update DomainDataGrant(%s/%s) error:%s", dg.Spec.GrantDomain, dg.Name, err.Error())
		}

		nlog.Infof("Update DomainDataGrant %s success, because %s spec is not match to %s", dg.Name, dg.Spec.GrantDomain, dg.Spec.Author)
		return true, nil
	}
	return false, nil
}

func (c *Controller) syncDomainDataHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("get DomainDataGrant(%s/%s) error", namespace, name)
		return nil
	}
	dd, err := c.domaindataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Not found DomainData(%s/%s), may be deleted", namespace, name)
			return nil
		}
		return fmt.Errorf("get DomainData(%s/%s) error:%s", namespace, name, err.Error())
	}

	if namespace == dd.Spec.Author {
		dd, err = c.domaindataLister.DomainDatas(dd.Spec.Author).Get(name)
		if err != nil {
			return fmt.Errorf("get DomainData(%s/%s) error:%s", dd.Spec.Author, name, err.Error())
		}

		r, _ := labels.NewRequirement(common.LabelOwnerReferences, selection.Equals, []string{dd.Name})
		dds, _ := c.domaindataLister.List(labels.NewSelector().Add(*r))
		for _, d := range dds {
			if !reflect.DeepEqual(dd.Spec, d.Spec) {
				dcopy := d.DeepCopy()
				dcopy.Spec = dd.Spec
				_, err = c.kusciaClient.KusciaV1alpha1().DomainDatas(dcopy.Namespace).Update(c.ctx, dcopy, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				nlog.Infof("Update DomainData %s/%s, because %s/%s update", dcopy.Namespace, dcopy.Name, d.Namespace, d.Name)
			}
		}
	}

	return nil
}

func (c *Controller) ensureDomainData(dg *v1alpha1.DomainDataGrant) error {
	domain, err := c.kusciaClient.KusciaV1alpha1().Domains().Get(c.ctx, dg.Spec.GrantDomain, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get domain %v err %v", dg.Spec.GrantDomain, err)
	}
	// 1. role is partner and master domain not empty => partner master domain
	// 2. role is partner and master domain empty => grant domain
	// 3. role is not partner => grant domain
	destDomain := dg.Spec.GrantDomain
	if domain.Spec.Role == v1alpha1.Partner && domain.Spec.MasterDomain != "" {
		destDomain = domain.Spec.MasterDomain
	}
	_, err = c.kusciaClient.KusciaV1alpha1().DomainDatas(destDomain).Get(c.ctx, dg.Spec.DomainDataID, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		dd, err := c.kusciaClient.KusciaV1alpha1().DomainDatas(dg.Namespace).Get(c.ctx, dg.Spec.DomainDataID, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't found domaindata %v/%v", dg.Namespace, dg.Spec.DomainDataID)
		}

		ddCopy := resources.ExtractDomainDataSpec(dd)
		ddCopy.Namespace = destDomain
		ddCopy.Labels[common.LabelOwnerReferences] = dd.Name
		ddCopy.Labels[common.LabelDomainDataVendor] = common.DomainDataVendorGrant
		if domain.Spec.Role == v1alpha1.Partner {
			ddCopy.Annotations[common.InitiatorAnnotationKey] = dg.Spec.Author
			ddCopy.Annotations[common.InterConnKusciaPartyAnnotationKey] = dg.Spec.GrantDomain
		}
		ddCopy.Spec.Vendor = common.DomainDataVendorGrant
		_, err = c.kusciaClient.KusciaV1alpha1().DomainDatas(destDomain).Create(c.ctx, ddCopy, metav1.CreateOptions{})
		nlog.Infof("Create DomainData %s/%s, because grant %s to %s", dg.Spec.GrantDomain, dg.Spec.DomainDataID, dg.Spec.Author, dg.Spec.GrantDomain)
		return err
	}
	return err
}

func (c *Controller) Name() string {
	return controllerName
}

func (c *Controller) doValidate(dg *v1alpha1.DomainDataGrant) error {
	if dg.Spec.Author == "" {
		return fmt.Errorf("validate DomainDataGrant(%s/%s) err, author cant be null", dg.Namespace, dg.Name)
	}
	if dg.Spec.GrantDomain == "" {
		return fmt.Errorf("validate DomainDataGrant(%s/%s) err, grantDomain cant be null", dg.Namespace, dg.Name)
	}
	if dg.Spec.GrantDomain == dg.Spec.Author {
		return fmt.Errorf("grantDomain cant be Author, skip sync DomainDataGrant(%s/%s)", dg.Namespace, dg.Name)
	}
	return nil
}

func (c *Controller) checkLabels(dg *v1alpha1.DomainDataGrant) (bool, error) {
	dgCopy := dg.DeepCopy()
	needUpdate := false
	if dgCopy.Labels == nil {
		dgCopy.Labels = map[string]string{}
	}
	if dgCopy.Annotations == nil {
		dgCopy.Annotations = map[string]string{}
	}

	if _, ok := dgCopy.Labels[common.LabelInterConnProtocolType]; !ok {
		dgCopy.Labels[common.LabelInterConnProtocolType] = "kuscia"
		needUpdate = true
	}

	if _, ok := dgCopy.Annotations[common.InitiatorAnnotationKey]; !ok {
		dgCopy.Annotations[common.InitiatorAnnotationKey] = dgCopy.Spec.Author
		needUpdate = true
	}

	if _, ok := dgCopy.Labels[common.LabelDomainDataID]; !ok {
		dgCopy.Labels[common.LabelDomainDataID] = dgCopy.Spec.DomainDataID
		needUpdate = true
	}
	if needUpdate {
		_, err := c.kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Update(c.ctx, dgCopy, metav1.UpdateOptions{})
		return true, err
	}
	return false, nil
}

func (c *Controller) verify(dg *v1alpha1.DomainDataGrant) error {
	phase := v1alpha1.GrantReady
	msg := ""

	if dg.Spec.Signature != "" {
		dm, err := c.domainLister.Get(dg.Spec.Author)
		if err != nil {
			c.updateStatus(dg, v1alpha1.GrantUnavailable, getDomainCertErrorStr)
			return fmt.Errorf("get domain %s cert error,err:%s", dg.Spec.Author, err.Error())
		}
		if dm.Spec.Cert == "" {
			c.updateStatus(dg, v1alpha1.GrantUnavailable, getDomainCertErrorStr)
			return fmt.Errorf("get domain %s cert error, because cert is empty", dg.Spec.Author)
		}
		pubKey, err := getPublickeyFromCert(dm.Spec.Cert)
		if err != nil {
			c.updateStatus(dg, v1alpha1.GrantUnavailable, getDomainCertErrorStr)
			return fmt.Errorf("get domain %s cert error,err:%s", dg.Spec.Author, err.Error())
		}
		dgcopy := dg.DeepCopy()
		dgcopy.Spec.Signature = ""
		bs, _ := json.Marshal(dgcopy.Spec)
		h := sha256.New()
		h.Write(bs)
		digest := h.Sum(nil)
		sign, err := base64.StdEncoding.DecodeString(dg.Spec.Signature)
		if err != nil {
			phase = v1alpha1.GrantUnavailable
			msg = "Verify error"
			return c.updateStatus(dg, phase, msg)
		}
		err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest, sign)
		if err != nil {
			phase = v1alpha1.GrantUnavailable
			msg = "Verify error"
			return c.updateStatus(dg, phase, msg)
		}
	}
	if dg.Spec.Limit != nil {
		if dg.Spec.Limit.ExpirationTime != nil {
			if time.Since(dg.Spec.Limit.ExpirationTime.Time) > 0 {
				phase = v1alpha1.GrantUnavailable
				msg = "Expirated"
			}
		}
		if dg.Spec.Limit.UseCount != 0 {
			if len(dg.Status.UseRecords) >= dg.Spec.Limit.UseCount {
				phase = v1alpha1.GrantUnavailable
				msg = "UseCount is exhausted"
			}
		}
	}
	return c.updateStatus(dg, phase, msg)
}

func (c *Controller) updateStatus(dg *v1alpha1.DomainDataGrant, phase v1alpha1.GrantPhase, msg string) error {
	if phase == dg.Status.Phase && msg == dg.Status.Message {
		return nil
	}
	dg = dg.DeepCopy()
	dg.Status.Phase = phase
	dg.Status.Message = msg
	_, err := c.kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).UpdateStatus(c.ctx, dg, metav1.UpdateOptions{})
	return err
}

func getPublickeyFromCert(certString string) (*rsa.PublicKey, error) {
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

	return rsaPub, nil
}
