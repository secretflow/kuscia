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
	if err := cfg.Config.Decode(&ci.EnvConfig); err != nil {
		return err
	}
	return nil
}

// CanExec implements the hook.Handler interface.
func (ci *envImport) CanExec(obj interface{}, point hook.Point) bool {
	if !ci.initialized {
		return false
	}

	if point != hook.PointGenerateRunContainerOptions {
		return false
	}

	_, ok := obj.(*hook.RunContainerOptionsObj)
	return ok
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
func (ci *envImport) ExecHook(obj interface{}, point hook.Point) (*hook.Result, error) {
	if !ci.initialized {
		return nil, fmt.Errorf("plugin cert-issuance is not initialized")
	}

	rObj, ok := obj.(*hook.RunContainerOptionsObj)
	if !ok {
		return nil, fmt.Errorf("can't convert object to pod")
	}

	imageMeta := &ImageMeta{}
	resp, err := rObj.ImageService.ImageStatus(context.Background(), &runtimeapi.ImageSpec{Image: rObj.Container.
		Image}, true)
	if err != nil {
		nlog.Warn("can't get image meta info, image name is", rObj.Container.Image)
	} else {
		infos := resp.GetInfo()["info"]
		if err = json.Unmarshal([]byte(infos), &imageMeta); err != nil {
			nlog.Warn("can't get image meta info, image name is", rObj.Container.Image)
		}
	}

	for _, env := range ci.EnvConfig.EnvList {
		match := true
		for _, s := range env.Selectors {
			match = matchImageMeta(imageMeta, s)
			if ci.EnvConfig.UsePodLabels && !match {
				match = matchPodLabels(rObj.Pod, s)
			}
			if !match {
				break
			}
		}
		if match {
			rObj.Opts.Envs = append(rObj.Opts.Envs, env.Envs...)
		}
	}

	return &hook.Result{}, nil
}
