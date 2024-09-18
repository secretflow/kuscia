// Copyright 2024 Ant Group Co., Ltd.
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

package pod

import (
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	resourcetest "github.com/secretflow/kuscia/pkg/agent/resource/testing"
)

func createTestCRIProvider(t *testing.T) *CRIProvider {
	tmpDir := t.TempDir()
	defaultAgentConfig := config.DefaultStaticAgentConfig()
	dep := &CRIProviderDependence{
		Namespace:        "default",
		NodeIP:           "127.0.0.1",
		RootDirectory:    tmpDir,
		StdoutDirectory:  path.Join(tmpDir, "var/stdout"),
		AllowPrivileged:  false,
		NodeName:         "test-node",
		EventRecorder:    nil,
		ResourceManager:  resourcetest.FakeResourceManager("default"),
		PodStateProvider: nil,
		PodSyncHandler:   nil,
		StatusManager:    nil,
		Runtime:          config.ProcessRuntime,
		CRIProviderCfg:   &defaultAgentConfig.Provider.CRI,
	}
	cp, err := NewCRIProvider(dep)
	assert.NoError(t, err)
	return cp.(*CRIProvider)
}

func TestCRIProvider_GenerateRunContainerOptions(t *testing.T) {
	t.Parallel()

	cp := createTestCRIProvider(t)
	pods := createTestPods()

	tests := []struct {
		pod  *v1.Pod
		opts *pkgcontainer.RunContainerOptions
	}{
		{
			pod: &pods[0],
			opts: &pkgcontainer.RunContainerOptions{
				Envs: []pkgcontainer.EnvVar{
					{
						Name:  "ENV_1",
						Value: "VALUE_1",
					},
				},
				Mounts: []pkgcontainer.Mount{
					{
						Name:          "volume-1",
						HostPath:      "/tmp/config",
						ContainerPath: "/home/config",
					},
					{
						Name: "volume-1",

						HostPath:      "/tmp/config",
						ContainerPath: "/root/config",
					},
					{
						Name:          "storage",
						HostPath:      cp.GetStorageDir(),
						ContainerPath: cp.GetStorageDir(),
					},
				},
				Hostname: "pod-1",
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			err := cp.volumeManager.MountVolumesForPod(tt.pod)
			assert.NoError(t, err)
			opts, _, err := cp.GenerateRunContainerOptions(tt.pod, &tt.pod.Spec.Containers[0], "192.168.0.1", nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.opts, opts)
		})
	}

}

func createTestPods() []v1.Pod {
	return []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: "default",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "container-1",
						Env: []v1.EnvVar{
							{
								Name:  "ENV_1",
								Value: "VALUE_1",
							},
						},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "volume-1",
								MountPath: "/home/config",
							},
							{
								Name:      "volume-1",
								MountPath: "./config",
							},
						},
						WorkingDir: "/root",
					},
				},
				Volumes: []v1.Volume{
					{
						Name: "volume-1",
						VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{
								Path: "/tmp/config",
							},
						},
					},
				},
			},
		},
	}

}
