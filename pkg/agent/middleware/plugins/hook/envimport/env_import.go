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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Selector struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type EnvList struct {
	Envs      []container.EnvVar `yaml:"envs"`
	Selectors []Selector         `yaml:"selectors"`
}

const (
	pluginNameEnv = "env-import"
)

func Register() {
	plugin.Register(pluginNameEnv, &envImport{})
}

type EnvConfig struct {
	UsePodLabels bool      `yaml:"usePodLabels"`
	EnvList      []EnvList `yaml:"envList"`
}

type envImport struct {
	EnvConfig   EnvConfig
	initialized bool
}

type Config struct {
	Labels map[string]string `json:"Labels"`
}

type ImageSpec struct {
	Config Config `json:"config"`
}

type ImageMeta struct {
	ImageSpec ImageSpec `json:"imageSpec"`
}

// Type implements the plugin.Plugin interface.
func (ci *envImport) Type() string {
	return hook.PluginType
}

// Init implements the plugin.Plugin interface.
func (ci *envImport) Init(dependencies *plugin.Dependencies, cfg *config.PluginCfg) error {
	ci.initialized = true
	hook.Register(pluginNameEnv, ci)
	ci.EnvConfig.UsePodLabels = true
	if err := cfg.Config.Decode(&ci.EnvConfig); err != nil {
		return err
	}
	return nil
}

// CanExec implements the hook.Handler interface.
func (ci *envImport) CanExec(ctx hook.Context) bool {
	if !ci.initialized {
		return false
	}

	if ctx.Point() != hook.PointGenerateContainerOptions && ctx.Point() != hook.PointK8sProviderSyncPod {
		return false
	}

	return true
}

func matchImageMeta(imageMeta *ImageMeta, s Selector) bool {
	if imageMeta == nil {
		return false
	}
	if imageMeta.ImageSpec.Config.Labels[s.Key] != s.Value {
		return false
	}
	return true
}
func matchPodLabels(pod *corev1.Pod, s Selector) bool {
	if pod == nil || pod.Labels == nil {
		return false
	}
	return pod.Labels[s.Key] == s.Value
}

// ExecHook implements the hook.Handler interface.
// It renders the configuration template and writes the generated real configuration content to a new file/directory.
// The value of hostPath will be replaced by the new file/directory path.
func (ci *envImport) ExecHook(ctx hook.Context) (*hook.Result, error) {
	if !ci.initialized {
		return nil, fmt.Errorf("plugin cert-issuance is not initialized")
	}

	switch ctx.Point() {
	case hook.PointGenerateContainerOptions:
		gCtx, ok := ctx.(*hook.GenerateContainerOptionContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := ci.handleGenerateOptionContext(gCtx); err != nil {
			return nil, err
		}
	case hook.PointK8sProviderSyncPod:
		syncPodCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := ci.handleSyncPodContext(syncPodCtx); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid point %v", ctx.Point())
	}

	return &hook.Result{}, nil

}

func (ci *envImport) handleGenerateOptionContext(ctx *hook.GenerateContainerOptionContext) error {
	imageMeta := &ImageMeta{}
	resp, err := ctx.ImageService.ImageStatus(context.Background(), &runtimeapi.ImageSpec{Image: ctx.Container.
		Image}, true)
	if err != nil {
		nlog.Warn("can't get image meta info, image name is", ctx.Container.Image)
	} else {
		infos := resp.GetInfo()["info"]
		if err = json.Unmarshal([]byte(infos), &imageMeta); err != nil {
			nlog.Warn("can't get image meta info, image name is", ctx.Container.Image)
		}
	}

	for _, env := range ci.EnvConfig.EnvList {
		match := true
		for _, s := range env.Selectors {
			match = matchImageMeta(imageMeta, s)
			if ci.EnvConfig.UsePodLabels && !match {
				match = matchPodLabels(ctx.Pod, s)
			}
			if !match {
				break
			}
		}
		if match {
			ctx.Opts.Envs = append(ctx.Opts.Envs, env.Envs...)
		}
	}

	return nil
}

func (ci *envImport) handleSyncPodContext(ctx *hook.K8sProviderSyncPodContext) error {
	pod := ctx.BkPod
	for _, envs := range ci.EnvConfig.EnvList {
		match := true
		for _, s := range envs.Selectors {
			match = matchPodLabels(pod, s)
			if !match {
				break
			}
		}
		if match {
			for i := range pod.Spec.Containers {
				ctr := &pod.Spec.Containers[i]
				for _, e := range envs.Envs {
					ctr.Env = append(ctr.Env, corev1.EnvVar{Name: e.Name, Value: e.Value})
				}
			}
		}
	}

	return nil
}
