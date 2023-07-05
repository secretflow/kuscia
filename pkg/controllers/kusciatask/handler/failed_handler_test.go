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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciascheme "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/scheme"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestFailedHandler_Handle(t *testing.T) {
	assert.NoError(t, kusciascheme.AddToScheme(scheme.Scheme))

	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task-1",
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
	go kubeInformersFactory.Start(wait.NeverStop)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-task-controller"})
	deps := &Dependencies{
		KubeClient:      kubeClient,
		KusciaClient:    kusciaClient,
		PodsLister:      kubeInformersFactory.Core().V1().Pods().Lister(),
		ConfigMapLister: kubeInformersFactory.Core().V1().ConfigMaps().Lister(),
		Recorder:        recorder,
		TrgLister:       trgInformer.Lister(),
	}

	finishHandler := NewFinishedHandler(deps)
	failedHandler := NewFailedHandler(deps, finishHandler)
	needUpdate, err := failedHandler.Handle(kt)
	assert.NoError(t, err)
	assert.Equal(t, true, needUpdate)
}

func TestSetTaskResourceGroupFailed(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	trg := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{},
	}

	kusciaClient := kusciafake.NewSimpleClientset(trg)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
	trgInformer.Informer().GetStore().Add(trg)

	h := &FailedHandler{
		kusciaClient: kusciaClient,
		trgLister:    trgInformer.Lister(),
	}

	h.setTaskResourceGroupFailed(kt)
	curTrg, err := kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(context.Background(), trg.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, kusciaapisv1alpha1.TaskResourceGroupPhaseFailed, curTrg.Status.Phase)
}
