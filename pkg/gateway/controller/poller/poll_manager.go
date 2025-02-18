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

	"github.com/stathat/consistent"
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
	processPeriod          = time.Second
	defaultSyncPeriod      = 10 * time.Minute
	maxRetries             = 16
	defaultGatewayTickTime = 15 * time.Second

	svcPollQueueName = "service-poll-queue"
	drPollQueueName  = "domain-route-poll-queue"
)

type PollState int

type PollManager struct {
	serviceLister           corelisters.ServiceLister
	serviceListerSynced     cache.InformerSynced
	domainRouteLister       kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced cache.InformerSynced
	gatewayLister           kuscialistersv1alpha1.GatewayLister
	gatewayListerSynced     cache.InformerSynced
	svcQueue                workqueue.RateLimitingInterface
	drQueue                 workqueue.RateLimitingInterface
	gatewayName             string
	selfNamespace           string
	isMaster                bool

	client      *http.Client
	pollers     map[string]map[string]*PollClient
	pollersLock sync.Mutex

	receiverDomains     map[string]bool
	receiverDomainsLock sync.RWMutex

	consist *consistent.Consistent
}

func NewPollManager(isMaster bool, selfNamespace, gatewayName string, serviceInformer corev1informers.ServiceInformer,
	domainRouteInformer kusciaextv1alpha1.DomainRouteInformer, gatewayInformer kusciaextv1alpha1.GatewayInformer) (*PollManager, error) {
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
		gatewayLister:           gatewayInformer.Lister(),
		gatewayListerSynced:     gatewayInformer.Informer().HasSynced,
		svcQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), svcPollQueueName),
		drQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), drPollQueueName),
		client:                  client,
		pollers:                 map[string]map[string]*PollClient{},
		receiverDomains:         map[string]bool{},
		consist:                 consistent.New(),
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
	if !cache.WaitForNamedCacheSync("poll manager", stopCh, pm.serviceListerSynced, pm.domainRouteListerSynced, pm.gatewayListerSynced) {
		nlog.Fatal("Failed to wait for caches to sync")
	}

	gatewayList, err := pm.gatewayLister.List(labels.Everything())
	if err != nil {
		nlog.Fatalf("Failed to list gateways: %v", err)
	}
	for _, gateway := range gatewayList {
		if isGatewayAlive(gateway) {
			pm.consist.Add(gateway.Name)
		}
	}

	nlog.Info("Starting poll manager ")
	for i := 0; i < workers; i++ {
		go wait.Until(pm.runServiceWorker, processPeriod, stopCh)
		go wait.Until(pm.runDomainRouteWorker, processPeriod, stopCh)
	}
	go pm.runGatewayTicker(stopCh)

	<-stopCh
	nlog.Info("Shutting down poll manager")
}

func (pm *PollManager) runServiceWorker() {
	for queue.HandleQueueItem(context.Background(), svcPollQueueName, pm.svcQueue, pm.syncHandleService, maxRetries) {
	}
}

func (pm *PollManager) runDomainRouteWorker() {
	for queue.HandleQueueItem(context.Background(), drPollQueueName, pm.drQueue, pm.syncHandleDomainRoute, maxRetries) {
	}
}

func (pm *PollManager) runGatewayTicker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(defaultGatewayTickTime)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if err := pm.updateAliveGateways(); err != nil {
				nlog.Errorf("Failed to update alive gateways: %v", err)
			}
		}
	}
}

func (pm *PollManager) updateAliveGateways() error {
	gatewayList, err := pm.gatewayLister.List(labels.Everything())
	if err != nil {
		return err
	}

	members := pm.consist.Members()
	aliveGateways := map[string]bool{}
	for _, gateway := range gatewayList {
		if isGatewayAlive(gateway) {
			aliveGateways[gateway.Name] = true
		}
	}

	membersChanged := false
	if len(members) != len(aliveGateways) {
		nlog.Infof("Current gateway member count is %d, while alive gateway count is %d", len(members), len(aliveGateways))
		membersChanged = true
	} else {
		for _, member := range members {
			if !aliveGateways[member] {
				nlog.Infof("Gateway Member %q is not alive now", member)
				membersChanged = true
			}
		}
	}

	if !membersChanged {
		return nil
	}

	var newMembers []string
	for gatewayName := range aliveGateways {
		newMembers = append(newMembers, gatewayName)
	}

	nlog.Infof("Gateway Members changed, old: %+v new: %+v", members, newMembers)

	pm.consist.Set(newMembers)
	pm.handleAllServices()

	return nil
}

func isGatewayAlive(gateway *kusciaapisv1alpha1.Gateway) bool {
	return time.Since(gateway.Status.HeartbeatTime.Time) <= 2*defaultGatewayTickTime
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

func (pm *PollManager) syncHandleService(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		nlog.Debugf("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	domains := map[string]bool{}

	if isDefaultService(key) {
		domains = pm.buildReceiverDomainsAll(name)
	} else {
		service, err := pm.serviceLister.Services(namespace).Get(name)
		if k8serrors.IsNotFound(err) {
			pm.setPollConnection(name, nil)
			return nil
		} else if err != nil {
			return err
		}

		domains = pm.buildReceiverDomainsByService(service)
	}

	pm.setPollConnection(name, domains)

	return nil
}

func (pm *PollManager) buildReceiverDomainsByService(service *v1.Service) map[string]bool {
	if service.Annotations != nil && service.Annotations[common.AccessDomainAnnotationKey] != "" {
		retDomains := map[string]bool{}

		domains := strings.Split(service.Annotations[common.AccessDomainAnnotationKey], ",")
		for _, domain := range domains {
			if !pm.isReceiverDomain(domain) {
				nlog.Debugf("Need not to poll domain %s", domain)
				continue
			}

			if pm.needSelfPoll(service.Name, domain) {
				retDomains[domain] = true
			}
		}
		return retDomains
	}

	return pm.buildReceiverDomainsAll(service.Name)
}

func (pm *PollManager) buildReceiverDomainsAll(serviceName string) map[string]bool {
	retDomains := map[string]bool{}

	receiverDomains := pm.getReceiverDomains()
	for _, domain := range receiverDomains {
		if pm.needSelfPoll(serviceName, domain) {
			retDomains[domain] = true
		}
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
		pm.needNotPollSource(sourceDomain)
		return nil
	} else if err != nil {
		return err
	}

	if dr.Spec.Transit == nil || dr.Spec.Transit.TransitMethod != kusciaapisv1alpha1.TransitMethodReverseTunnel {
		pm.needNotPollSource(dr.Spec.Source)
		return nil
	}

	if err := pm.needPollSource(dr); err != nil {
		return err
	}

	return nil
}

func (pm *PollManager) needNotPollSource(sourceDomain string) {
	pm.receiverDomainsLock.Lock()
	defer pm.receiverDomainsLock.Unlock()

	if pm.receiverDomains[sourceDomain] {
		delete(pm.receiverDomains, sourceDomain)
		pm.handleAllServices()
	}
}

func (pm *PollManager) needPollSource(dr *kusciaapisv1alpha1.DomainRoute) error {
	pm.receiverDomainsLock.Lock()
	defer pm.receiverDomainsLock.Unlock()

	source := dr.Spec.Source
	if pm.receiverDomains[source] {
		nlog.Debugf("DomainRoute %s has triggered polling", dr.Name)
		return nil
	}
	pm.receiverDomains[source] = true

	nlog.Infof("DomainRoute %s triggered polling", dr.Name)

	pm.handleAllServices()

	return nil
}

func (pm *PollManager) handleAllServices() {
	services, err := pm.serviceLister.Services(pm.selfNamespace).List(labels.Everything())
	if err != nil {
		nlog.Errorf("Failed to list services: %v", err)
		return
	}

	for _, service := range services {
		if !serviceFilter(service) {
			continue
		}

		pm.enqueueService(service)
	}

	defaultServices := []string{utils.ServiceHandshake}
	if pm.isMaster {
		defaultServices = append(defaultServices, utils.ServiceAPIServer)
	}

	for _, service := range defaultServices {
		pm.svcQueue.Add(service)
	}
}

func (pm *PollManager) needSelfPoll(serviceName string, receiverDomain string) bool {
	key := buildPollerKey(serviceName, receiverDomain)
	expected, err := pm.consist.Get(key)
	if err != nil {
		nlog.Errorf("Unable to select consistent hash ring for key %q: %v", key, err)
		return false
	}

	if expected != pm.gatewayName {
		nlog.Debugf("Unexpected gateway %q for key %q, expected gateway %q, skip", pm.gatewayName, key, expected)
		return false
	}

	return true
}

func (pm *PollManager) setPollConnection(serviceName string, domains map[string]bool) {
	pm.pollersLock.Lock()
	defer pm.pollersLock.Unlock()

	if domains == nil {
		domains = map[string]bool{}
	}

	svcPoller, ok := pm.pollers[serviceName]
	if !ok {
		if len(domains) == 0 {
			return
		}
		svcPoller = map[string]*PollClient{}
		pm.pollers[serviceName] = svcPoller
	}

	var addDomainList, removeDomainList []string
	for domain := range svcPoller {
		if _, ok := domains[domain]; !ok {
			removeDomainList = append(removeDomainList, domain)
		}
	}
	for domain := range domains {
		if _, ok := svcPoller[domain]; !ok {
			addDomainList = append(addDomainList, domain)
		}
	}

	for _, domain := range removeDomainList {
		if poller, ok := svcPoller[domain]; ok {
			poller.Stop()
			delete(svcPoller, domain)
			nlog.Infof("Remove poll connection: %v", buildPollerKey(serviceName, domain))
		}
	}
	for _, domain := range addDomainList {
		poller := newPollClient(pm.client, serviceName, domain, pm.selfNamespace)
		svcPoller[domain] = poller
		poller.Start()

		nlog.Infof("Add poll connection: %v", buildPollerKey(serviceName, domain))
	}

	if len(svcPoller) == 0 {
		delete(pm.pollers, serviceName)

		nlog.Infof("Poll connections of service %q were all removed", serviceName)
	}
}

func (pm *PollManager) getReceiverDomains() []string {
	pm.receiverDomainsLock.RLock()
	defer pm.receiverDomainsLock.RUnlock()

	var domains []string
	for domain := range pm.receiverDomains {
		domains = append(domains, domain)
	}

	return domains
}

func (pm *PollManager) isReceiverDomain(domain string) bool {
	pm.receiverDomainsLock.RLock()
	defer pm.receiverDomainsLock.RUnlock()

	return pm.receiverDomains[domain]
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

func isDefaultService(service string) bool {
	return service == utils.ServiceHandshake || service == utils.ServiceAPIServer
}
