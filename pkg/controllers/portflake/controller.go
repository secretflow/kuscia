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

package portflake

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/controllers/portflake/port"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	DefaultScanPortProvidersInterval = 60

	controllerName = "portflake-controller"
)

type ResourceEvent int

const (
	ResourceEventAdd = iota + 1
	ResourceEventDelete
)

type PortController struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	kubeInformerFactory informers.SharedInformerFactory

	podInformer        corev1informers.PodInformer
	podSynced          cache.InformerSynced
	deploymentInformer appsinformers.DeploymentInformer
	deploymentSynced   cache.InformerSynced

	chReady chan struct{}

	scanPortProvidersInterval time.Duration
}

func NewController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		config.KubeClient,
		5*time.Minute,

		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = common.LabelController
		}),
	)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	c := &PortController{
		kubeInformerFactory:       kubeInformerFactory,
		podInformer:               podInformer,
		podSynced:                 podInformer.Informer().HasSynced,
		deploymentInformer:        deploymentInformer,
		deploymentSynced:          deploymentInformer.Informer().HasSynced,
		chReady:                   make(chan struct{}),
		scanPortProvidersInterval: time.Duration(DefaultScanPortProvidersInterval) * time.Second,
	}
	c.ctx, c.cancel = context.WithCancel(ctx)

	_, _ = c.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			c.handlePodEvent(pod, ResourceEventAdd)
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			// When a delete is dropped, the relist will notice a pod in the store not
			// in the list, leading to the insertion of a tombstone object which contains
			// the deleted key/value. Note that this value might be stale. If the pod
			// changed labels the new ReplicaSet will not be woken up till the periodic
			// resync.
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					nlog.Errorf("Couldn't get object from tombstone %#v", obj)
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					nlog.Errorf("Tombstone contained object that is not a pod %#v", obj)
					return
				}
			}

			c.handlePodEvent(pod, ResourceEventDelete)
		},
	})

	_, _ = c.deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			c.handleDeploymentEvent(deployment, ResourceEventAdd)
		},
		DeleteFunc: func(obj interface{}) {
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					nlog.Errorf("Couldn't get object from tombstone %#v", obj)
					return
				}
				deployment, ok = tombstone.Obj.(*appsv1.Deployment)
				if !ok {
					nlog.Errorf("Tombstone contained object that is not a deployment %#v", obj)
					return
				}
			}

			c.handleDeploymentEvent(deployment, ResourceEventDelete)
		},
	})

	return c
}

func (c *PortController) scanPortProviders() {
	port.ScanPortProviders()
}

func (c *PortController) handleDeploymentEvent(deployment *appsv1.Deployment, event ResourceEvent) {
	nlog.Debugf("Receive deployment event [%v], namespace=%v, pod name=%v", event, deployment.Namespace, deployment.Name)

	// fetch pod ports
	var podPorts []int
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, containerPort := range container.Ports {
			podPorts = append(podPorts, int(containerPort.ContainerPort))
		}
	}

	portProvider := port.GetPortProvider(deployment.Namespace)
	switch event {
	case ResourceEventAdd:
		portProvider.AddIndeed(buildDeploymentOwnerName(deployment.Name), podPorts)
	case ResourceEventDelete:
		portProvider.DeleteIndeed(buildDeploymentOwnerName(deployment.Name), podPorts)
	default:
		nlog.Errorf("Unsupported deployment event: %v", event)
	}
}

func (c *PortController) handlePodEvent(pod *corev1.Pod, event ResourceEvent) {
	if pod.Labels == nil || pod.Labels[common.LabelController] != common.ControllerKusciaTask {
		return
	}

	nlog.Debugf("Receive pod event [%v], namespace=%v, pod name=%v", event, pod.Namespace, pod.Name)

	// fetch pod ports
	var podPorts []int
	for _, container := range pod.Spec.Containers {
		for _, containerPort := range container.Ports {
			podPorts = append(podPorts, int(containerPort.ContainerPort))
		}
	}

	portProvider := port.GetPortProvider(pod.Namespace)
	switch event {
	case ResourceEventAdd:
		portProvider.AddIndeed(buildPodOwnerName(pod.Name), podPorts)
	case ResourceEventDelete:
		portProvider.DeleteIndeed(buildPodOwnerName(pod.Name), podPorts)
	default:
		nlog.Errorf("Unsupported pod event: %v", event)
	}
}

func (c *PortController) controlLoop() {
	scanPortProvidersTicker := time.NewTicker(c.scanPortProvidersInterval)
	defer scanPortProvidersTicker.Stop()

	for {
		select {
		case <-scanPortProvidersTicker.C:
			c.scanPortProviders()
		case <-c.ctx.Done():
			return // exit
		}
	}
}

func (c *PortController) Run(i int) error {
	nlog.Infof("Starting %v", c.Name())

	c.kubeInformerFactory.Start(c.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", c.Name())

	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.podSynced, c.deploymentSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Infof("Controller %v is ready", c.Name())
	close(c.chReady)

	c.controlLoop()

	return nil
}

func (c *PortController) Ready() <-chan struct{} {
	return c.chReady
}

func (c *PortController) Stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *PortController) Name() string {
	return controllerName
}

func buildDeploymentOwnerName(deploymentName string) string {
	return fmt.Sprintf("deployment:%s", deploymentName)
}

func buildPodOwnerName(podName string) string {
	return fmt.Sprintf("pod:%s", podName)
}
