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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

// Generate sourceApiserver config
func createApiserverSourceCfg(namespace string, nodeName types.NodeName, kubeClient kubernetes.Interface) *InitConfig {
	cfg := &InitConfig{
		Namespace:  namespace,
		NodeName:   nodeName,
		KubeClient: kubeClient,
		SourceCfg: &config.SourceCfg{
			Apiserver: config.ApiserverSourceCfg{
				KubeConnCfg: config.KubeConnCfg{
					KubeconfigFile: "/tmp/kubeconfig",
				},
			},
		},
	}

	return cfg
}

// Generate new instance of test pod with the same initial value.
func getTestPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "default",
		},
	}
}

func TestApiserverPodEvent(t *testing.T) {
	testPod := getTestPod()
	kubeClient := fake.NewSimpleClientset(testPod)
	cfg := createApiserverSourceCfg("default", "test-node", kubeClient)
	updates := make(chan kubetypes.PodUpdate, 5)

	s := newApiserverSource(cfg, updates)

	go func() {
		update := <-updates
		assert.Equal(t, update.Op, kubetypes.ADD)
		assert.Equal(t, len(update.Pods), 1)
		assert.Equal(t, update.Pods[0].Name, "foo")

		update = <-updates
		assert.Equal(t, update.Op, kubetypes.UPDATE)

		update = <-updates
		assert.Equal(t, update.Op, kubetypes.REMOVE)
	}()

	stopCh := make(chan struct{})

	assert.NoError(t, s.run(stopCh))

	// update pod
	testPod.Labels = map[string]string{
		"op": "update",
	}
	ctx := context.Background()
	_, err := kubeClient.CoreV1().Pods("default").Update(ctx, testPod, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// delete pod
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: new(int64),
		Preconditions:      metav1.NewUIDPreconditions(string(testPod.UID)),
	}
	err = kubeClient.CoreV1().Pods("default").Delete(ctx, testPod.Name, deleteOptions)
	assert.NoError(t, err)
}
