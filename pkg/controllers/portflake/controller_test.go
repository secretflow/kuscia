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
	"testing"
	"time"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers/portflake/port"
)

func TestPortController(t *testing.T) {
	fk8s := fake.NewSimpleClientset()
	iFactory := kubeinformers.NewSharedInformerFactoryWithOptions(fk8s, 1*time.Minute)
	podInformer := iFactory.Core().V1().Pods()
	deploymentInformer := iFactory.Apps().V1().Deployments()
	pc := &PortController{
		podInformer: podInformer,
		podSynced: func() bool {
			return true
		},
		deploymentInformer: deploymentInformer,
		deploymentSynced: func() bool {
			return true
		},
		chReady:                   make(chan struct{}),
		scanPortProvidersInterval: 10 * time.Millisecond,
		kubeInformerFactory:       iFactory,
		ctx:                       context.Background(),
	}

	go func() {
		assert.NilError(t, pc.Run(0))
	}()

	pp := port.GetPortProvider("ns_a")
	ports, err := pp.Allocate(2)
	assert.NilError(t, err)
	assert.Equal(t, 2, pp.PortCount())
	assert.Equal(t, 2, pp.PortToVerifyCount())

	pod := &corev1.Pod{}
	pod.Name = "pod_a"
	pod.Namespace = "ns_a"
	pod.Labels = map[string]string{
		common.LabelController: "111",
	}
	pod.Spec.Containers = []corev1.Container{
		{
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: ports[0],
				},
				{
					ContainerPort: ports[1],
				},
			},
		},
	}

	pc.handlePodEvent(pod, ResourceEventAdd)
	assert.Equal(t, 2, pp.PortCount())
	assert.Equal(t, 0, pp.PortToVerifyCount())
	pc.handlePodEvent(pod, ResourceEventDelete)
	assert.Equal(t, 0, pp.PortCount())
	assert.Equal(t, 0, pp.PortToVerifyCount())

	ports, err = pp.Allocate(2)
	assert.NilError(t, err)
	assert.Equal(t, 2, pp.PortCount())
	assert.Equal(t, 2, pp.PortToVerifyCount())

	pc.Stop()
}
