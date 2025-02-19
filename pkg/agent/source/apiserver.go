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

package source

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/utils/format"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type sourceApiserver struct {
	podInformerFactory kubeinformers.SharedInformerFactory
	podInformer        corev1informers.PodInformer
	updates            chan<- kubetypes.PodUpdate
}

func newApiserverSource(cfg *InitConfig, updates chan<- kubetypes.PodUpdate) *sourceApiserver {
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		cfg.KubeClient, 0,
		kubeinformers.WithNamespace(cfg.Namespace),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", string(cfg.NodeName)).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	s := &sourceApiserver{
		podInformerFactory: podInformerFactory,
		podInformer:        podInformer,
		updates:            updates,
	}

	_, _ = podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if ok {
				nlog.Infof("Receive pod %q add event from apiserver", format.Pod(pod))
				s.updatePods([]*corev1.Pod{pod}, kubetypes.ADD)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod, ok := newObj.(*corev1.Pod)
			if ok {
				nlog.Infof("Receive pod %q update event from apiserver", format.Pod(pod))
				s.updatePods([]*corev1.Pod{pod}, kubetypes.UPDATE)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if ok {
				nlog.Infof("Receive pod %q delete event from apiserver", format.Pod(pod))
				s.updatePods([]*corev1.Pod{pod}, kubetypes.REMOVE)
			}
		},
	})

	return s
}

func (s *sourceApiserver) run(stopCh <-chan struct{}) error {
	nlog.Info("Start running apiserver source")

	go s.podInformerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting to do work.
	if ok := cache.WaitForCacheSync(stopCh, s.podInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for pod caches to sync")
	}

	nlog.Info("Pod Informer: cache sync finished")

	return nil
}

func (s *sourceApiserver) updatePods(pods []*corev1.Pod, op kubetypes.PodOperation) {
	s.updates <- kubetypes.PodUpdate{Pods: pods, Op: op, Source: kubetypes.ApiserverSource}
}
