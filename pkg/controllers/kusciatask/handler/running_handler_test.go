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

package handler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestRunningHandler_Handle(t *testing.T) {
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
		},
	}

	testPod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{common.LabelTaskResourceGroup: "task"},
			Name:      "pod-01",
			Namespace: "ns-a",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	testPod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{common.LabelTaskResourceGroup: "task"},
			Name:      "pod-01",
			Namespace: "ns-b",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	testKusciaTask := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task",
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{
			PodStatuses: map[string]*kusciaapisv1alpha1.PodStatus{
				"ns-a/pod-01": {
					Namespace: "ns-a",
					PodName:   "pod-01",
				},
				"ns-b/pod-01": {
					Namespace: "ns-b",
					PodName:   "pod-01",
				},
			},
		},
	}

	tests := []struct {
		kusciatask *kusciaapisv1alpha1.KusciaTask
		trg        *kusciaapisv1alpha1.TaskResourceGroup
		pods       []runtime.Object
		wantPhase  kusciaapisv1alpha1.KusciaTaskPhase
	}{
		{
			kusciatask: testKusciaTask,
			trg:        trg1,
			pods: []runtime.Object{
				testPod1,
				testPod2,
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			kusciatask: testKusciaTask,
			trg:        trg2,
			pods: []runtime.Object{
				testPod1,
				testPod2,
			},
			wantPhase: kusciaapisv1alpha1.TaskFailed,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("TestCase %d", i), func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset(tt.pods...)
			kusciaClient := kusciafake.NewSimpleClientset()
			kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
			trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
			trgInformer.Informer().GetStore().Add(tt.trg)
			podInformer := kubeInformersFactory.Core().V1().Pods()
			for _, ttp := range tt.pods {
				podInformer.Informer().GetStore().Add(ttp)
			}
			go kubeInformersFactory.Start(wait.NeverStop)
			deps := Dependencies{
				KubeClient:   kubeClient,
				KusciaClient: kusciaClient,
				TrgLister:    trgInformer.Lister(),
				PodsLister:   kubeInformersFactory.Core().V1().Pods().Lister(),
			}
			h := NewRunningHandler(&deps)
			_, err := h.Handle(tt.kusciatask)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPhase, tt.kusciatask.Status.Phase)
		})
	}
}
