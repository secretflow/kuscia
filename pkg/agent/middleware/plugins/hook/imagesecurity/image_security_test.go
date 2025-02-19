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

package imagesecurity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/agent/provider/pod"
	"github.com/secretflow/kuscia/pkg/common"
)

const (
	testImageID = "sha256:f1c20d8cb5c4c69d3997527e4912e794ba3cd7fa26bfaf6afa1383697c80ea9a"
)

func setupTestConfigRender(t *testing.T) *imageSecurity {
	configYaml := `
name: "image-security"
`
	cfg := &config.PluginCfg{}
	assert.NoError(t, yaml.Unmarshal([]byte(configYaml), cfg))

	agentConfig := config.DefaultAgentConfig(common.DefaultKusciaHomePath())

	dep := &plugin.Dependencies{
		AgentConfig: agentConfig,
	}

	is := &imageSecurity{}
	assert.Equal(t, hook.PluginType, is.Type())
	assert.NoError(t, is.Init(context.Background(), dep, cfg))

	return is
}

func TestImageSecurity_ExecHook(t *testing.T) {
	is := setupTestConfigRender(t)

	imageService := &fakeImageManagerService{}
	criProvider := &pod.CRIProvider{ImageManagerService: imageService}

	tests := []struct {
		Pod        *v1.Pod
		Terminated bool
	}{
		{
			&v1.Pod{
				Spec: v1.PodSpec{Containers: []v1.Container{{Image: "test:0.1"}}},
			},
			false,
		},
		{
			&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{common.ImageIDAnnotationKey: "sha256:abc"}},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Image: "test:0.1"}}},
			},
			true,
		},
		{
			&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{common.ImageIDAnnotationKey: "abc"}},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Image: "test:0.1"}}},
			},
			false,
		},
		{
			&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{common.ImageIDAnnotationKey: testImageID}},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Image: "test:0.1"}}},
			},
			false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("No.%d", i), func(t *testing.T) {
			ctx := &hook.PodAdditionContext{Pod: tt.Pod, PodProvider: criProvider}
			assert.True(t, is.CanExec(ctx))
			result, err := is.ExecHook(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.Terminated, result.Terminated)
		})

	}
}

type fakeImageManagerService struct {
}

func (f *fakeImageManagerService) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.
	Image, error) {
	return nil, nil
}

func (f *fakeImageManagerService) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec, verbose bool) (*runtimeapi.ImageStatusResponse, error) {
	return &runtimeapi.ImageStatusResponse{Image: &runtimeapi.Image{Id: testImageID}}, nil
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
