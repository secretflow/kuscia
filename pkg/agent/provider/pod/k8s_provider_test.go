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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	frameworktest "github.com/secretflow/kuscia/pkg/agent/framework/testing"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	resourcetest "github.com/secretflow/kuscia/pkg/agent/resource/testing"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/election"
)

func createTestK8sProvider(t *testing.T, cfg *config.K8sProviderCfg, rm *resource.KubeResourceManager) *K8sProvider {
	bkClient := fake.NewSimpleClientset()

	podProviderDep := &K8sProviderDependence{
		NodeName:        "test-node",
		BkClient:        bkClient,
		PodSyncHandler:  frameworktest.FakeSyncHandler{},
		ResourceManager: rm,
		K8sProviderCfg:  cfg,
		Recorder:        &record.FakeRecorder{},
	}

	kp, err := NewK8sProvider(podProviderDep)
	assert.NoError(t, err)
	return kp
}

func TestK8sProvider_Start(t *testing.T) {
	rm := resourcetest.FakeResourceManager("test-namespace")

	kp := createTestK8sProvider(t, &config.K8sProviderCfg{
		Namespace: "bk-namespace",
	}, rm)

	newLeaderCh := make(chan struct{}, 1)
	startedLeadingCh := make(chan struct{}, 1)
	stoppedLeadingCh := make(chan struct{}, 1)
	kp.leaderElector = election.NewElector(
		kp.bkClient,
		kp.nodeName,
		election.WithLockType(resourcelock.ConfigMapsLeasesResourceLock),
		election.WithOnNewLeader(func(s string) {
			t.Log("new leader", s)
			newLeaderCh <- struct{}{}
		}),
		election.WithOnStartedLeading(func(ctx context.Context) {
			t.Log("started leading")
			startedLeadingCh <- struct{}{}
		}),
		election.WithOnStoppedLeading(func() {
			t.Log("stopped leading")
			stoppedLeadingCh <- struct{}{}
		}))
	assert.True(t, kp.leaderElector != nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		assert.NoError(t, kp.Start(ctx))
	}()
	<-newLeaderCh
	<-startedLeadingCh
	cancel()
	<-stoppedLeadingCh
}

func TestK8sProvider_SyncAndKillPod(t *testing.T) {
	podConfig := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-template",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"config.yaml": "aa=${AA}",
		},
	}
	podSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		StringData: map[string]string{
			"secret": "123",
		},
	}

	rm := resourcetest.FakeResourceManager("test-namespace", podConfig, podSecret)
	rootDir := t.TempDir()
	resolveConfig := filepath.Join(rootDir, "resolve.conf")
	assert.NoError(t, os.WriteFile(resolveConfig, []byte("nameserver 127.0.0.1"), 0644))

	cfg := &config.K8sProviderCfg{
		Namespace: "bk-namespace",
		DNS: config.DNSCfg{
			ResolverConfig: resolveConfig,
		},
		RuntimeClassName: "runc",
	}
	kp := createTestK8sProvider(t, cfg, rm)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "abc",
			Name:      "pod01",
			Namespace: "test-namespace",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "ctr01",
					Command: []string{"sleep 60"},
					Image:   "aa/bb:001",
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "config-template",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "config-template",
							},
						},
					},
				},
				{
					Name: "test-secret",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "test-secret",
						},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	assert.NoError(t, kp.SyncPod(context.Background(), pod, nil, nil))

	go kp.kubeInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), kp.podsSynced, kp.configMapSynced, kp.secretSynced) {
		t.Fatal("timeout waiting for caches to sync")
	}

	newPod, err := kp.bkClient.CoreV1().Pods(kp.bkNamespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, kp.bkNamespace, newPod.Namespace)
	assert.Equal(t, 3, len(newPod.Spec.Volumes))
	assert.Equal(t, "config-template", newPod.Spec.Volumes[0].Name)
	assert.Equal(t, "test-secret", newPod.Spec.Volumes[1].Name)
	assert.Equal(t, "resolv-config", newPod.Spec.Volumes[2].Name)
	assert.Equal(t, "runc", *newPod.Spec.RuntimeClassName)

	newPodConfig, err := kp.configMapLister.Get("pod01-config-template")
	assert.NoError(t, err)
	assert.Equal(t, "abc", newPodConfig.Labels[common.LabelPodUID])
	assert.Equal(t, "aa=${AA}", newPodConfig.Data["config.yaml"])

	newResolveConfig, err := kp.configMapLister.Get("pod01-resolv-config")
	assert.NoError(t, err)
	assert.Equal(t, "abc", newPodConfig.Labels[common.LabelPodUID])
	assert.Equal(t, "nameserver 127.0.0.1", newResolveConfig.Data["resolv.conf"])

	newSecret, err := kp.secretLister.Get("pod01-test-secret")
	assert.NoError(t, err)
	assert.Equal(t, "abc", newSecret.Labels[common.LabelPodUID])
	assert.Equal(t, "123", newSecret.StringData["secret"])

	// kill pod
	assert.NoError(t, kp.KillPod(context.Background(), pod, pkgcontainer.Pod{}, nil))
	_, err = kp.bkClient.CoreV1().Pods(kp.bkNamespace).Get(context.Background(), newPod.Name, metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))

	cancel()
}

func TestNormalizeSubResourceMeta(t *testing.T) {
	resourceNameLimit = 10
	tests := []struct {
		meta         *metav1.ObjectMeta
		ownerPodName string
		wantMetaName string
	}{
		{
			meta: &metav1.ObjectMeta{
				Name: "aaa",
			},
			ownerPodName: "pod-01",
			wantMetaName: "pod-01-aaa",
		},
		{
			meta: &metav1.ObjectMeta{
				Name: "aaa",
			},
			ownerPodName: "pod-001",
			wantMetaName: "dd2d7b3cd0",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			normalizeSubResourceMeta(tt.meta, tt.ownerPodName)
			assert.Equal(t, tt.wantMetaName, tt.meta.Name)
		})
	}
}
