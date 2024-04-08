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
	"strings"

	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/common"
)

func Register() {
	plugin.Register(common.PluginNameImageSecurity, &imageSecurity{})
}

type imageSecurityConfig struct {
}

type imageSecurity struct {
	config imageSecurityConfig
}

// Type implements the plugin.Plugin interface.
func (cr *imageSecurity) Type() string {
	return hook.PluginType
}

// Init implements the plugin.Plugin interface.
func (cr *imageSecurity) Init(dependencies *plugin.Dependencies, cfg *config.PluginCfg) error {
	if err := cfg.Config.Decode(&cr.config); err != nil {
		return err
	}

	hook.Register(common.PluginNameImageSecurity, cr)
	return nil
}

// CanExec implements the hook.Handler interface.
func (cr *imageSecurity) CanExec(ctx hook.Context) bool {
	switch ctx.Point() {
	case hook.PointPodAddition:
		criCtx, ok := ctx.(*hook.PodAdditionContext)
		if !ok {
			return false
		}
		_, ok = criCtx.PodProvider.(internalapi.ImageManagerService)
		if !ok {
			return false
		}
		return true
	default:
		return false
	}
}

// ExecHook implements the hook.Handler interface.
func (cr *imageSecurity) ExecHook(ctx hook.Context) (*hook.Result, error) {
	result := &hook.Result{}

	criCtx, ok := ctx.(*hook.PodAdditionContext)
	if !ok {
		return nil, fmt.Errorf("failed to convert ctx to PodAdditionContext")
	}
	imageManagerService, ok := criCtx.PodProvider.(internalapi.ImageManagerService)
	if !ok {
		return nil, fmt.Errorf("failed to convert pod provider to ImageManagerService")
	}

	if criCtx.Pod.Annotations == nil {
		return result, nil
	}

	imageID, ok := criCtx.Pod.Annotations[common.ImageIDAnnotationKey]
	if !ok || imageID == "" || !strings.HasPrefix(imageID, "sha256:") {
		return result, nil
	}

	for _, c := range criCtx.Pod.Spec.Containers {
		imageStatus, err := imageManagerService.ImageStatus(context.Background(), &runtimeapi.ImageSpec{Image: c.Image}, false)
		if err != nil {
			return nil, err
		}
		expectedImageID := imageStatus.Image.Id

		// adapt process runtime
		if !strings.HasPrefix(imageStatus.Image.Id, "sha256:") && len(imageStatus.Image.RepoTags) > 0 {
			expectedImageID = imageStatus.Image.RepoTags[0]
		}

		if expectedImageID != imageID {
			return &hook.Result{
				Terminated: true,
				Msg:        fmt.Sprintf("image %q ID mismatch, expected %q, actual %q", c.Image, expectedImageID, imageID),
			}, nil
		}
	}

	return result, nil
}
