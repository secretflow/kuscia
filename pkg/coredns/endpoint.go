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

package coredns

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	maxRetries     = 15
	controllerName = "kuscia-coredns-controller"
)

// EndpointsController sync namespaces' services. For each service
// - If `ObjectMeta.Annotations` contains `cname: gateway`, insert A record with gateway IP.
// - Otherwise, insert A records with endpoints IPs.
type EndpointsController struct {
	namespace             string
	ctx                   context.Context
	cancel                context.CancelFunc
	instance              *KusciaCoreDNS
	serviceLister         corelister.ServiceLister
	serviceListerSynced   cache.InformerSynced
	endpointsLister       corelister.EndpointsLister
	endpointsListerSynced cache.InformerSynced
	queue                 workqueue.RateLimitingInterface
}

// NewEndpointsController create a new *EndpointsController
func NewEndpointsController(ctx context.Context, kubeClient kubernetes.Interface, instance *KusciaCoreDNS, namespace string) controllers.IController {
	sharedInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, defaultSyncPeriod, informers.WithNamespace(namespace))
	serviceInformer := sharedInformers.Core().V1().Services()
	endpointsInformer := sharedInformers.Core().V1().Endpoints()

	c := &EndpointsController{
		namespace:             namespace,
		instance:              instance,
		serviceLister:         serviceInformer.Lister(),
		serviceListerSynced:   serviceInformer.Informer().HasSynced,
		endpointsLister:       endpointsInformer.Lister(),
		endpointsListerSynced: endpointsInformer.Informer().HasSynced,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "endpoints"),
	}
	c.ctx, c.cancel = context.WithCancel(ctx)

	sharedInformers.Start(ctx.Done())
	_, _ = serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addObj,
			UpdateFunc: c.updateObj,
			DeleteFunc: c.addObj,
		},
		defaultSyncPeriod,
	)

	_, _ = endpointsInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addObj,
			UpdateFunc: c.updateObj,
			DeleteFunc: c.addObj,
		},
		defaultSyncPeriod,
	)

	if !cache.WaitForNamedCacheSync("endpoints", ctx.Done(), c.serviceListerSynced, c.endpointsListerSynced) {
		nlog.Fatalf("Wait for service cache sync fail, namespace: %s", namespace)
	}
	return c
}

func (c *EndpointsController) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.queue.ShutDown()
}

func (c *EndpointsController) addObj(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.queue)
}

func (c *EndpointsController) updateObj(old, cur interface{}) {
	queue.EnqueueObjectWithKey(cur, c.queue)
}

// Run begins watching and syncing.
func (c *EndpointsController) Run(workers int) error {
	for queue.HandleQueueItem(c.ctx, controllerName, c.queue, c.syncHandler, maxRetries) {
	}
	return nil
}

func (c *EndpointsController) syncHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	domain := fmt.Sprintf("%s.%s", name, namespace)

	service, err := c.serviceLister.Services(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		c.instance.Cache.Delete(name)
		c.instance.Cache.Delete(domain)
		return nil
	}

	if err != nil {
		nlog.Errorf("unable to retrieve service %v from store: %v", key, err)
		return err
	}

	if service.Labels[common.LabelLoadBalancer] == string(common.DomainRouteLoadBalancer) {
		c.instance.Cache.Set(name, []string{c.instance.EnvoyIP}, 0)
		c.instance.Cache.Set(domain, []string{c.instance.EnvoyIP}, 0)
		return nil
	}

	if service.Spec.Type == v1.ServiceTypeExternalName {
		c.instance.Cache.Set(name, []string{service.Spec.ExternalName}, 0)
		c.instance.Cache.Set(domain, []string{service.Spec.ExternalName}, 0)
		return nil
	}

	endpoints, err := c.endpointsLister.Endpoints(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		c.instance.Cache.Delete(name)
		c.instance.Cache.Delete(domain)
		return nil
	}

	if err != nil {
		nlog.Errorf("unable to retrieve endpoints %v from store: %v", key, err)
		return err
	}

	var hosts []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			hosts = append(hosts, address.IP)
		}
	}

	c.instance.Cache.Set(name, hosts, 0)
	c.instance.Cache.Set(domain, hosts, 0)
	return nil
}

func (c *EndpointsController) Name() string {
	return "kuscia-coredns-endpoint-controller"
}
