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

func (f *fakeImageManagerService) ImageFsInfo(ctx context.Context) ([]*runtimeapi.FilesystemUsage, error) {
	return nil, nil
}

func TestEnvImport(t *testing.T) {
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
			envs: nil,
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
			envs: nil,
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
			envs: nil,
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
			err = ci.Init(nil, &pluginCfg)
			assert.NoError(t, err)

			obj := &hook.RunContainerOptionsObj{
				Pod:          &pod,
				Container:    &pod.Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				ImageService: &fakeImageManagerService{},
			}
			assert.True(t, ci.CanExec(obj, hook.PointGenerateRunContainerOptions))

			_, err = ci.ExecHook(obj, hook.PointGenerateRunContainerOptions)
			assert.NoError(t, err)
			assert.Equal(t, tt.envs, obj.Opts.Envs)
		})
	}
}
