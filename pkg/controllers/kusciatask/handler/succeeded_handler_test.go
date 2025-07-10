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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/dependencies"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestSucceededHandler_Handle(t *testing.T) {
	t.Parallel()
	testKusciaTask := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			UID: "abc",
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	go kubeInformersFactory.Start(wait.NeverStop)

	deps := dependencies.Dependencies{
		KubeClient:      kubeClient,
		KusciaClient:    kusciaClient,
		TrgLister:       kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups().Lister(),
		PodsLister:      kubeInformersFactory.Core().V1().Pods().Lister(),
		ConfigMapLister: kubeInformersFactory.Core().V1().ConfigMaps().Lister(),
	}

	finishHandler := NewFinishedHandler(&deps)
	succeededHandler := NewSucceededHandler(finishHandler)
	needUpdate, err := succeededHandler.Handle(testKusciaTask)
	assert.NoError(t, err)
	assert.Equal(t, true, needUpdate)
}
