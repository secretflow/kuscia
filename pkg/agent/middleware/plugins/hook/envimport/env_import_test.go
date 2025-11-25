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

package envimport

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
)

type fakeImageManagerService struct {
}

func (f *fakeImageManagerService) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.
	Image, error) {
	return nil, nil
}

func (f *fakeImageManagerService) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec,
	verbose bool) (*runtimeapi.ImageStatusResponse, error) {
	return &runtimeapi.ImageStatusResponse{Info: map[string]string{"info": `{"imageSpec":{"config":{"Labels":{"maintainer":"secretflow-contact@service.alipay.com"}}}}`}}, nil
}

func (f *fakeImageManagerService) PullImage(ctx context.Context, image *runtimeapi.ImageSpec,
	auth *runtimeapi.AuthConfig,
	podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return "", nil
}

func (f *fakeImageManagerService) RemoveImage(ctx context.Context, image *runtimeapi.ImageSpec) error {
	return nil
}

func (f *fakeImageManagerService) ImageFsInfo(ctx context.Context) (*runtimeapi.ImageFsInfoResponse, error) {
	return &runtimeapi.ImageFsInfoResponse{}, nil
}

func TestEnvImport_ExecHookWithGenerateOptionContext(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"name": "alice",
				"role": "test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "default-container",
					Image: "secretflow-anolis8",
				},
			},
		},
	}

	tests := []struct {
		data string
		envs []pkgcontainer.EnvVar
	}{
		{
			data: `
usePodLabels: false
envList:
- envs:
  - name: image
    value: secretflow
  selectors:
  - key: secretflow-anolis8
    value: image`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: false
envList:
- envs:
  - name: image
    value: secretflow
  selectors:
  - key: maintainer
    value: secretflow-contact@service.alipay.com`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "image",
					Value: "secretflow",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: true
envList:
- envs:
  - name: test
    value: test
  selectors:
  - key: name
    type: pod
    value: alice`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: true
envList:
- envs:
  - name: test
    value: test
  selectors:
  - key: name
    type: pod
    value: bob`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: true
envList:
- envs:
  - name: test
    value: test
  - name: image
    value: secretflow
  selectors:
  - key: name
    value: alice
  - key: maintainer
    value: secretflow-contact@service.alipay.com`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "image",
					Value: "secretflow",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: false
envList:
- envs:
  - name: test
    value: test
  - name: image
    value: secretflow
  selectors:
  - key: name
    value: alice
  - key: maintainer
    value: secretflow-contact@service.alipay.com`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
usePodLabels: false
envList:
- envs:
  - name: test
    value: test
  - name: image
    value: secretflow`,
			envs: []pkgcontainer.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "image",
					Value: "secretflow",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Case-%d", i), func(t *testing.T) {
			pluginCfg := config.PluginCfg{
				Name: "env-import",
			}
			err := yaml.Unmarshal([]byte(tt.data), &pluginCfg.Config)
			assert.NoError(t, err)

			ci := &envImport{}
			err = ci.Init(context.Background(), nil, &pluginCfg)
			assert.NoError(t, err)

			ctx := &hook.GenerateContainerOptionContext{
				Pod:          &pod,
				Container:    &pod.Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				ImageService: &fakeImageManagerService{},
			}
			assert.True(t, ci.CanExec(ctx))

			_, err = ci.ExecHook(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.envs, ctx.Opts.Envs)
		})
	}
}

func TestEnvImport_ExecHookWithSyncPodContext(t *testing.T) {
	tests := []struct {
		data string
		envs []corev1.EnvVar
	}{
		{
			data: `
envList:
- envs:
  - name: test
    value: test
  selectors:
  - key: name
    type: pod
    value: alice`,
			envs: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
envList:
- envs:
  - name: test
    value: test
  selectors:
  - key: name
    type: pod
    value: bob`,
			envs: []corev1.EnvVar{
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
		{
			data: `
envList:
- envs:
  - name: test
    value: test
  - name: image
    value: secretflow`,
			envs: []corev1.EnvVar{
				{
					Name:  "test",
					Value: "test",
				},
				{
					Name:  "image",
					Value: "secretflow",
				},
				{
					Name:  "CONTAINER_CPU_LIMIT",
					Value: "0.00",
				},
				{
					Name:  "CONTAINER_MEM_LIMIT",
					Value: "0Mi",
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Case-%d", i), func(t *testing.T) {
			pluginCfg := config.PluginCfg{
				Name: "env-import",
			}
			err := yaml.Unmarshal([]byte(tt.data), &pluginCfg.Config)
			assert.NoError(t, err)

			ci := &envImport{}
			err = ci.Init(context.Background(), nil, &pluginCfg)
			assert.NoError(t, err)

			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-0",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"name": "alice",
						"role": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "default-container",
							Image: "secretflow-anolis8",
						},
					},
				},
			}
			ctx := &hook.K8sProviderSyncPodContext{
				Pod:   &pod,
				BkPod: &pod,
			}
			assert.True(t, ci.CanExec(ctx))

			_, err = ci.ExecHook(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.envs, ctx.BkPod.Spec.Containers[0].Env)
		})
	}
}

func TestGetContainerResources(t *testing.T) {
	agentCpu := k8sresource.MustParse("4")
	agentMem := k8sresource.MustParse("8Gi")

	tests := []struct {
		name           string
		container      *corev1.Container
		expectedCPU    string
		expectedMemory string
	}{
		{
			name: "with both cpu and memory limits",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    k8sresource.MustParse("2"),
						corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
					},
				},
			},
			expectedCPU:    "2",
			expectedMemory: "4096Mi",
		},
		{
			name: "with only cpu limit",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: k8sresource.MustParse("1.5"),
					},
				},
			},
			expectedCPU:    "2",
			expectedMemory: "6553Mi", // 8Gi * 0.8 / (1024*1024)
		},
		{
			name: "with only memory limit",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("2Gi"),
					},
				},
			},
			expectedCPU:    "3.20",
			expectedMemory: "2048Mi",
		},
		{
			name:           "with no limits",
			container:      &corev1.Container{},
			expectedCPU:    "3.20",
			expectedMemory: "6553Mi",
		},
		{
			name: "with zero cpu limit",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: k8sresource.MustParse("0"),
					},
				},
			},
			expectedCPU:    "0",
			expectedMemory: "6553Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem := getContainerResources(tt.container, agentCpu, agentMem)
			assert.Equal(t, tt.expectedCPU, cpu)
			assert.Equal(t, tt.expectedMemory, mem)
		})
	}
}
