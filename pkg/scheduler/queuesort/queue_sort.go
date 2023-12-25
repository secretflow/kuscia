/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queuesort

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformer "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = "KusciaSort"

type KusciaSort struct {
	// trLister is TaskResource lister.
	trLister kuscialistersv1alpha1.TaskResourceLister
	// podLister is pod lister.
	podLister listerv1.PodLister
}

var _ framework.QueueSortPlugin = &KusciaSort{}

func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	kubeConfig := *handle.KubeConfig()
	kusciaClient := kusciaclientset.NewForConfigOrDie(&kubeConfig)
	kusciaInformerFactory := kusciainformer.NewSharedInformerFactory(kusciaClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	podInformer := handle.SharedInformerFactory().Core().V1().Pods()

	plugin := &KusciaSort{
		trLister:  trInformer.Lister(),
		podLister: podInformer.Lister(),
	}

	ctx := context.Background()
	kusciaInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), trInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("failed to wait for cache sync for task resource")
	}
	return plugin, nil
}

// Name returns name of the plugin.
func (*KusciaSort) Name() string {
	return Name
}

// Less is used to sort pods in the scheduling queue in the following order.
// 1. Compare the priorities of Pods.
// 2. Compare the initialization timestamps of TaskResource or Pods.
// 3. Compare the keys of Namespace/Pods: <namespace>/<podName>.
func (ks *KusciaSort) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	prio1 := corev1helpers.PodPriority(podInfo1.Pod)
	prio2 := corev1helpers.PodPriority(podInfo2.Pod)
	if prio1 != prio2 {
		return prio1 > prio2
	}

	creationTime1 := ks.getCreationTimestamp(podInfo1.Pod, podInfo1.InitialAttemptTimestamp)
	creationTime2 := ks.getCreationTimestamp(podInfo2.Pod, podInfo2.InitialAttemptTimestamp)
	if creationTime1.Equal(creationTime2) {
		return getNamespacedName(podInfo1.Pod) < getNamespacedName(podInfo2.Pod)
	}
	return creationTime1.Before(creationTime2)
}

// GetCreationTimestamp returns the creation time of a TaskResource or a pod.
func (ks *KusciaSort) getCreationTimestamp(pod *corev1.Pod, ts time.Time) time.Time {
	if pod.Labels != nil {
		trName := pod.Labels[kusciaapisv1alpha1.LabelTaskResource]
		if trName == "" {
			return ts
		}

		tr, _ := ks.trLister.TaskResources(pod.Namespace).Get(trName)
		if tr == nil {
			return ts
		}
		return tr.CreationTimestamp.Time
	}

	return ts
}

// getNamespacedName returns the namespaced name.
func getNamespacedName(obj metav1.Object) string {
	return fmt.Sprintf("%v/%v", obj.GetNamespace(), obj.GetName())
}
