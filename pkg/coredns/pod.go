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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// PodController sync namespace's pods. For each pod, if `PodSpec.subdomain` is specified,
// insert A record with FQDN "<hostname>.<subdomain>.<namespace>".
type PodController struct {
	instance        *KusciaCoreDNS
	ctx             context.Context
	cancel          context.CancelFunc
	podLister       corelister.PodLister
	podListerSynced cache.InformerSynced
}

// NewPodController create a new *PodController.
func NewPodController(ctx context.Context, kubeClient kubernetes.Interface, instance *KusciaCoreDNS, namespace string) controllers.IController {
	sharedInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, defaultSyncPeriod, informers.WithNamespace(namespace))
	podInformer := sharedInformers.Core().V1().Pods()

	c := &PodController{
		instance:        instance,
		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,
	}
	c.ctx, c.cancel = context.WithCancel(ctx)

	sharedInformers.Start(ctx.Done())
	podInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				pod, ok := obj.(*v1.Pod)
				if ok {
					return pod.Spec.Subdomain != ""
				}
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addPod,
				UpdateFunc: c.updatePod,
				DeleteFunc: c.deletePod,
			},
		},
		defaultSyncPeriod,
	)
	if !cache.WaitForNamedCacheSync("pod", ctx.Done(), c.podListerSynced) {
		nlog.Fatalf("Wait for pod cache sync fail, namespace: %s", namespace)
	}
	return c
}

func (c *PodController) addPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return
	}
	nlog.Debugf("Adding pod %s/%s", pod.Namespace, pod.Name)

	c.instance.Cache.Set(fmt.Sprintf("%s.%s", pod.Spec.Hostname, pod.Spec.Subdomain), []string{pod.Status.PodIP}, 0)
	c.instance.Cache.Set(fmt.Sprintf("%s.%s.%s", pod.Spec.Hostname, pod.Spec.Subdomain, pod.Namespace), []string{pod.Status.PodIP}, 0)
}

func (c *PodController) updatePod(old, cur interface{}) {
	c.addPod(cur)
}

func (c *PodController) deletePod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		nlog.Warnf("interface{} is %T, not *v1.Pod", obj)
		return
	}

	nlog.Debugf("Deleting pod %s/%s", pod.Namespace, pod.Name)
	c.instance.Cache.Delete(fmt.Sprintf("%s.%s", pod.Spec.Hostname, pod.Spec.Subdomain))
	c.instance.Cache.Delete(fmt.Sprintf("%s.%s.%s", pod.Spec.Hostname, pod.Spec.Subdomain, pod.Namespace))
}

func (c *PodController) Run(int) error {
	<-c.ctx.Done()
	return nil
}

func (c *PodController) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *PodController) Name() string {
	return "kuscia-coredns-pod-controller"
}
