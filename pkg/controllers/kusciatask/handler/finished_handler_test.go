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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func TestFinishedHandler_Handle(t *testing.T) {
	t.Parallel()
	testKusciaTask := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "abc",
			UID:  "111",
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{Phase: kusciaapisv1alpha1.TaskSucceeded},
	}

	testPod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-a",
			Name:      "pod-01",
			Labels: map[string]string{
				common.LabelTaskUID: "111",
			},
		},
		Status: v1.PodStatus{Phase: v1.PodSucceeded},
	}

	testPod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-a",
			Name:      "pod-02",
		},
	}

	testConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-a",
			Name:      "cm-01",
			Labels: map[string]string{
				common.LabelTaskUID: "111",
			},
		},
	}

	kubeClient := kubefake.NewSimpleClientset(testPod1, testPod2, testConfigMap)
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kubeInformersFactory.Start(wait.NeverStop)
	podInformer := kubeInformersFactory.Core().V1().Pods()
	configMapInformer := kubeInformersFactory.Core().V1().ConfigMaps()
	podInformer.Informer().GetStore().Add(testPod1)
	podInformer.Informer().GetStore().Add(testPod2)
	configMapInformer.Informer().GetStore().Add(testConfigMap)

	deps := &Dependencies{
		KubeClient:      kubeClient,
		KusciaClient:    kusciaClient,
		PodsLister:      kubeInformersFactory.Core().V1().Pods().Lister(),
		ConfigMapLister: kubeInformersFactory.Core().V1().ConfigMaps().Lister(),
	}
	h := NewFinishedHandler(deps)
	_, err := h.Handle(testKusciaTask)
	assert.NoError(t, err)

	_, err = kubeClient.CoreV1().Pods("ns-a").Get(context.Background(), "pod-01", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = kubeClient.CoreV1().Pods("ns-a").Get(context.Background(), "pod-02", metav1.GetOptions{})
	assert.NoError(t, err)

	_, err = kubeClient.CoreV1().ConfigMaps("ns-a").Get(context.Background(), "cm-01", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

}
