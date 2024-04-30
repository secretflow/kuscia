// Copyright 2024 Ant Group Co., Ltd.
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

package poller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	processPeriod     = time.Second
	defaultSyncPeriod = 10 * time.Minute
	maxRetries        = 16

	svcPollQueueName = "service-poll-queue"
	drPollQueueName  = "domain-route-poll-queue"
)

type PollState int

const (
	PollStateUnknown PollState = iota
	PollStateNotPoll
	PollStatePolling
)

type PollManager struct {
	serviceLister           corelisters.ServiceLister
	serviceListerSynced     cache.InformerSynced
	domainRouteLister       kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced cache.InformerSynced
	svcQueue                workqueue.RateLimitingInterface
	drQueue                 workqueue.RateLimitingInterface
	gatewayName             string
	selfNamespace           string
	isMaster                bool

	client      *http.Client
	pollers     map[string]map[string]*PollClient
	pollersLock sync.Mutex

	sourcePollState map[string]PollState
	pollStateLock   sync.RWMutex
}

func NewPollManager(isMaster bool, selfNamespace, gatewayName string, serviceInformer corev1informers.ServiceInformer, domainRouteInformer kusciaextv1alpha1.DomainRouteInformer) (*PollManager, error) {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
	}
	pm := &PollManager{
		isMaster:                isMaster,
		gatewayName:             gatewayName,
		selfNamespace:           selfNamespace,
		serviceLister:           serviceInformer.Lister(),
		serviceListerSynced:     serviceInformer.Informer().HasSynced,
		domainRouteLister:       domainRouteInformer.Lister(),
		domainRouteListerSynced: domainRouteInformer.Informer().HasSynced,
		svcQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), svcPollQueueName),
		drQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), drPollQueueName),
		client:                  client,
		pollers:                 map[string]map[string]*PollClient{},
		sourcePollState:         map[string]PollState{},
	}

	if err := pm.addServiceEventHandler(serviceInformer); err != nil {
		return nil, err
	}
	if err := pm.addDomainRouteEventHandler(domainRouteInformer); err != nil {
		return nil, err
	}

	return pm, nil
}

func (pm *PollManager) Run(workers int, stopCh <-chan struct{}) {
	defer func() {
		utilruntime.HandleCrash()
		pm.svcQueue.ShutDown()
		pm.drQueue.ShutDown()
	}()

	nlog.Info("Waiting for informer caches to sync")
	if !cache.WaitForNamedCacheSync("poll manager", stopCh, pm.serviceListerSynced, pm.domainRouteListerSynced) {
		nlog.Fatal("failed to wait for caches to sync")
	}

	nlog.Info("Starting poll manager ")
	for i := 0; i < workers; i++ {
		go wait.Until(pm.runServiceWorker, processPeriod, stopCh)
		go wait.Until(pm.runDomainRouteWorker, processPeriod, stopCh)
	}

	<-stopCh
	nlog.Info("Shutting down poll manager")
}

func (pm *PollManager) runServiceWorker() {
	for queue.HandleQueueItem(context.Background(), svcPollQueueName, pm.svcQueue, pm.syncHandlerService, maxRetries) {
	}
}

func (pm *PollManager) runDomainRouteWorker() {
	for queue.HandleQueueItem(context.Background(), drPollQueueName, pm.drQueue, pm.syncHandleDomainRoute, maxRetries) {
	}
}

func (pm *PollManager) addServiceEventHandler(serviceInformer corev1informers.ServiceInformer) error {
	_, err := serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Service:
					return serviceFilter(t)
				case cache.DeletedFinalStateUnknown:
					if svc, ok := t.Obj.(*v1.Service); ok {
						return serviceFilter(svc)
					}
					nlog.Errorf("Failed to decoding object, invalid type %T", t.Obj)
					return false
				default:
					nlog.Errorf("Invalid service object")
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					pm.enqueueService(obj)
				},
				UpdateFunc: func(_, newObj interface{}) {
					pm.enqueueService(newObj)
				},
				DeleteFunc: func(obj interface{}) {
					pm.enqueueService(obj)
				},
			},
		},
		defaultSyncPeriod,
	)

	return err
}

func serviceFilter(obj apismetav1.Object) bool {
	if objLabels := obj.GetLabels(); objLabels != nil {
		if objLabels[common.LabelPortScope] == string(kusciaapisv1alpha1.ScopeCluster) {
			return true
		}
	}
	return false
}

func (pm *PollManager) enqueueService(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, pm.svcQueue)
}

func (pm *PollManager) addDomainRouteEventHandler(domainRouteInformer kusciaextv1alpha1.DomainRouteInformer) error {
	_, err := domainRouteInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pm.handleDomainRouteObj(obj)
			},
			UpdateFunc: func(_, newObj interface{}) {
				pm.handleDomainRouteObj(newObj)
			},
			DeleteFunc: func(obj interface{}) {
				pm.handleDomainRouteObj(obj)
			},
		},
	)

	return err
}

func (pm *PollManager) handleDomainRouteObj(obj interface{}) {
	var domainRoute *kusciaapisv1alpha1.DomainRoute

	switch t := obj.(type) {
	case *kusciaapisv1alpha1.DomainRoute:
		domainRoute = t
	case cache.DeletedFinalStateUnknown:
		dr, ok := t.Obj.(*kusciaapisv1alpha1.DomainRoute)
		if !ok {
			nlog.Errorf("Failed to decoding object, invalid type %T", t.Obj)
			return
		}

		domainRoute = dr
	default:
		nlog.Errorf("Invalid domain route object")
		return
	}

	if !pm.domainRouteFilter(domainRoute) {
		return
	}

	key := buildDomainRouteKey(domainRoute.Name, domainRoute.Spec.Source)

	pm.drQueue.Add(key)
	nlog.Debugf("Enqueue key: %q", key)
}

func (pm *PollManager) domainRouteFilter(dr *kusciaapisv1alpha1.DomainRoute) bool {
	if dr.Spec.Destination != pm.selfNamespace {
		return false
	}

	return true
}

func (pm *PollManager) syncHandlerService(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		nlog.Debugf("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	service, err := pm.serviceLister.Services(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		pm.removePollConnection(name, nil)
		return nil
	} else if err != nil {
		return err
	}

	domains := pm.buildReceiverDomainsByService(service)

	pm.addPollConnection(name, domains)

	return nil
}

func (pm *PollManager) buildReceiverDomainsByService(service *v1.Service) []string {
	var retDomains []string
	if service.Annotations != nil && service.Annotations[common.AccessDomainAnnotationKey] != "" {
		domains := strings.Split(service.Annotations[common.AccessDomainAnnotationKey], ",")
		for _, domain := range domains {
			state := pm.getDomainPollState(domain)
			if !isPollingState(state) {
				nlog.Debugf("Need not to poll domain %s", domain)
				continue
			}

			// TODO Check if this instance needs to be processed
			retDomains = append(retDomains, domain)
		}
	} else {
		retDomains = pm.getPollingDomains()
	}

	return retDomains
}

func (pm *PollManager) syncHandleDomainRoute(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		nlog.Debugf("Finished syncing domain route %q (%v)", key, time.Since(startTime))
	}()

	name, sourceDomain, err := splitDomainRouteKey(key)
	if err != nil {
		return err
	}

	dr, err := pm.domainRouteLister.DomainRoutes(pm.selfNamespace).Get(name)
	if k8serrors.IsNotFound(err) {
		pm.domainRouteDeleted(sourceDomain)
		return nil
	} else if err != nil {
		return err
	}

	if dr.Spec.Transit == nil || dr.Spec.Transit.TransitMethod != kusciaapisv1alpha1.TransitMethodReverseTunnel {
		pm.needNotPollSource(dr)
		return nil
	}

	if err := pm.needPollSource(dr); err != nil {
		return err
	}

	return nil
}

func (pm *PollManager) getDomainPollState(domain string) PollState {
	pm.pollStateLock.RLock()
	defer pm.pollStateLock.RUnlock()

	return pm.sourcePollState[domain]
}

func (pm *PollManager) getPollingDomains() []string {
	pm.pollStateLock.RLock()
	defer pm.pollStateLock.RUnlock()

	var domains []string
	for domain, state := range pm.sourcePollState {
		if isPollingState(state) {
			domains = append(domains, domain)
		}
	}

	return domains
}

func (pm *PollManager) removeDRPollConnection(receiverDomain string) {
	pm.removePollConnection(utils.ServiceAPIServer, []string{receiverDomain})
	pm.removePollConnection(utils.ServiceHandshake, []string{receiverDomain})

	pm.handleServicesByDomainRoute(receiverDomain, func(serviceName string, receiverDomain string) {
		nlog.Infof("Prepare to remove poll connection %q", buildPollerKey(serviceName, receiverDomain))
		pm.removePollConnection(serviceName, []string{receiverDomain})
	})
}

func (pm *PollManager) domainRouteDeleted(sourceDomain string) {
	pm.pollStateLock.Lock()
	defer pm.pollStateLock.Unlock()

	if isPollingState(pm.sourcePollState[sourceDomain]) {
		delete(pm.sourcePollState, sourceDomain)
		pm.removeDRPollConnection(sourceDomain)
	}
}

func (pm *PollManager) needNotPollSource(dr *kusciaapisv1alpha1.DomainRoute) {
	pm.pollStateLock.Lock()
	defer pm.pollStateLock.Unlock()

	if isPollingState(pm.sourcePollState[dr.Spec.Source]) {
		pm.sourcePollState[dr.Spec.Source] = PollStateNotPoll
		pm.removeDRPollConnection(dr.Spec.Source)
	}
}

func (pm *PollManager) needPollSource(dr *kusciaapisv1alpha1.DomainRoute) error {
	pm.pollStateLock.Lock()
	defer pm.pollStateLock.Unlock()

	source := dr.Spec.Source
	if pm.sourcePollState[source] == PollStatePolling {
		nlog.Debugf("DomainRoute %s has triggered polling", dr.Name)
		return nil
	}
	pm.sourcePollState[source] = PollStatePolling

	receiverDomains := []string{source}

	if pm.isMaster {
		pm.addPollConnection(utils.ServiceAPIServer, receiverDomains)
	}
	pm.addPollConnection(utils.ServiceHandshake, receiverDomains)

	pm.handleServicesByDomainRoute(source, func(serviceName string, receiverDomain string) {
		nlog.Infof("Prepare to add poll connection %q", buildPollerKey(serviceName, receiverDomain))
		pm.addPollConnection(serviceName, []string{receiverDomain})
	})

	return nil
}

func (pm *PollManager) handleServicesByDomainRoute(sourceDomain string, pollConnectionHandler func(string, string)) {
	services, err := pm.serviceLister.Services(pm.selfNamespace).List(labels.Everything())
	if err != nil {
		nlog.Errorf("Failed to list services: %v", err)
		return
	}

	for _, service := range services {
		if !serviceFilter(service) {
			continue
		}
		if service.Annotations != nil && service.Annotations[common.AccessDomainAnnotationKey] != "" {
			domains := strings.Split(service.Annotations[common.AccessDomainAnnotationKey], ",")
			for _, domain := range domains {
				if domain == sourceDomain {
					pollConnectionHandler(service.Name, domain)
					break
				}
			}
		} else {
			pollConnectionHandler(service.Name, sourceDomain)
		}
	}
}

func (pm *PollManager) addPollConnection(serviceName string, domainList []string) {
	if len(domainList) == 0 {
		nlog.Debugf("No address for serviceName: %v, skip", serviceName)
		return
	}

	pm.pollersLock.Lock()
	defer pm.pollersLock.Unlock()

	svcPoller, ok := pm.pollers[serviceName]
	if !ok {
		svcPoller = map[string]*PollClient{}
		pm.pollers[serviceName] = svcPoller
	}

	for _, domain := range domainList {
		if _, ok := svcPoller[domain]; ok {
			continue
		}

		poller := newPollClient(pm.client, serviceName, domain)
		svcPoller[domain] = poller
		poller.Start()

		nlog.Infof("Add poll connection: %v", buildPollerKey(serviceName, domain))
	}
}

func (pm *PollManager) removePollConnection(serviceName string, domainList []string) {
	pm.pollersLock.Lock()
	defer pm.pollersLock.Unlock()

	svcPoller, ok := pm.pollers[serviceName]
	if !ok {
		return
	}

	if len(domainList) == 0 {
		for domain, poller := range svcPoller {
			poller.Stop()
			nlog.Infof("Remove poll connection: %v", buildPollerKey(serviceName, domain))
		}

		delete(pm.pollers, serviceName)
		return
	}

	for _, domain := range domainList {
		if poller, ok := svcPoller[domain]; ok {
			poller.Stop()
			delete(svcPoller, domain)
			nlog.Infof("Remove poll connection: %v", buildPollerKey(serviceName, domain))
		}
	}
}

func buildPollerKey(svcName string, receiverDomain string) string {
	return fmt.Sprintf("%s:%s", svcName, receiverDomain)
}

func buildDomainRouteKey(drName string, sourceDomain string) string {
	return fmt.Sprintf("%s:%s", drName, sourceDomain)
}

func splitDomainRouteKey(key string) (string, string, error) {
	arr := strings.Split(key, ":")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("invalid domain route key %q", key)
	}
	return arr[0], arr[1], nil
}

func isPollingState(state PollState) bool {
	return state == PollStatePolling
}
