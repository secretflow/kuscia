// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	resourcetest "github.com/secretflow/kuscia/pkg/agent/resource/testing"
	"github.com/secretflow/kuscia/pkg/common"
)

func TestK8sProvider_cleanupZombieResources(t *testing.T) {
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				common.LabelNodeNamespace: "default",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}
	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				common.LabelNodeNamespace: "default",
			},
		},
	}

	rootDir := t.TempDir()
	resolveConfig := filepath.Join(rootDir, "resolve.conf")
	assert.NoError(t, os.WriteFile(resolveConfig, []byte("nameserver 127.0.0.1"), 0644))

	cfg := &config.K8sProviderCfg{
		Namespace: "bk-namespace",
		DNS: config.DNSCfg{
			ResolverConfig: resolveConfig,
		},
	}
	rm := resourcetest.FakeResourceManager("test-namespace", node1, node2)

	kp := createTestK8sProvider(t, cfg, rm)
	kp.namespace = "default"

	kp.nodeName = "node1"
	pod1 := createTestPod("001", "default", "pod1")
	assert.NoError(t, kp.SyncPod(context.Background(), pod1, nil, nil))

	kp.nodeName = "node2"
	pod2 := createTestPod("002", "default", "pod2")
	assert.NoError(t, kp.SyncPod(context.Background(), pod2, nil, nil))

	kp.nodeName = "node3"
	pod3 := createTestPod("003", "default", "pod3")
	assert.NoError(t, kp.SyncPod(context.Background(), pod3, nil, nil))

	kp.nodeName = "node4"
	pod4 := createTestPod("004", "default", "pod4")
	assert.NoError(t, kp.SyncPod(context.Background(), pod4, nil, nil))
	assert.NoError(t, kp.KillPod(context.Background(), pod4, pkgcontainer.Pod{}, nil))

	ctx, cancel := context.WithCancel(context.Background())
	kp.kubeInformerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), kp.podsSynced, kp.configMapSynced) {
		t.Fatal("timeout waiting for caches to sync")
	}

	assertCachedBackendResource(t, kp, "pod1", true, true)
	assertCachedBackendResource(t, kp, "pod2", true, true)
	assertCachedBackendResource(t, kp, "pod3", true, true)
	assertCachedBackendResource(t, kp, "pod4", false, true)

	assert.NoError(t, kp.cleanupZombieResources(ctx))

	assertBackendResource(t, kp, "pod1", true)
	assertBackendResource(t, kp, "pod2", false)
	assertBackendResource(t, kp, "pod3", false)
	assertBackendResource(t, kp, "pod3", false)

	cancel()
}

func createTestPod(uid types.UID, namespace, name string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       uid,
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "ctr01",
					Command: []string{"sleep 60"},
					Image:   "aa/bb:001",
				},
			},
		},
	}
	return pod
}

func assertCachedBackendResource(t *testing.T, kp *K8sProvider, podName string, podExist, cmExist bool) {
	_, err := kp.podLister.Get(podName)
	assert.Equal(t, podExist, err == nil)

	_, err = kp.configMapLister.Get(fmt.Sprintf("%s-resolv-config", podName))
	assert.Equal(t, cmExist, err == nil)
}

func assertBackendResource(t *testing.T, kp *K8sProvider, podName string, podExist bool) {
	_, err := kp.bkClient.CoreV1().Pods(kp.bkNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	assert.Equal(t, podExist, err == nil)
}
