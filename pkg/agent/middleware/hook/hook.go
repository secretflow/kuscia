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

package hook

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	internalapi "k8s.io/cri-api/pkg/apis"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	PointMakeMounts = iota + 1
	PointGenerateContainerOptions
	PointK8sProviderSyncPod
)

const (
	PluginType = "hook"
)

type Point int

type Context interface {
	Point() Point
}

type K8sProviderSyncPodContext struct {
	Pod             *v1.Pod
	BkPod           *v1.Pod
	ResourceManager *resource.KubeResourceManager
	Configmaps      []*v1.ConfigMap
	Secrets         []*v1.Secret
}

func (c *K8sProviderSyncPodContext) Point() Point {
	return PointK8sProviderSyncPod
}

type MakeMountsContext struct {
	Pod           *v1.Pod
	Container     *v1.Container
	HostPath      *string
	Mount         *v1.VolumeMount
	Envs          []pkgcontainer.EnvVar
	PodVolumesDir string
}

func (c *MakeMountsContext) Point() Point {
	return PointMakeMounts
}

type GenerateContainerOptionContext struct {
	Pod          *v1.Pod
	Container    *v1.Container
	Opts         *pkgcontainer.RunContainerOptions
	PodIPs       []string
	ContainerDir string
	ImageService internalapi.ImageManagerService
}

func (c *GenerateContainerOptionContext) Point() Point {
	return PointGenerateContainerOptions
}

type Result struct {
	Terminated bool
	Msg        string
}

type Handler interface {
	CanExec(ctx Context) bool
	ExecHook(ctx Context) (*Result, error)
}

var (
	handlers = map[string]Handler{}
	orders   []string
)

// Register register hook handler.
func Register(name string, h Handler) {
	if _, ok := handlers[name]; ok {
		return
	}

	handlers[name] = h
	orders = append(orders, name)
}

// Execute traverse all handlers according to priority to execute the hook function.
func Execute(ctx Context) error {
	for _, name := range orders {
		h := handlers[name]
		if ok := h.CanExec(ctx); !ok {
			continue
		}

		result, err := h.ExecHook(ctx)
		if err != nil {
			return fmt.Errorf("plugin hook.%v execute failed, detail-> %v", name, err)
		}

		nlog.Debugf("Execute plugin hook.%v succeed, result=%+v", name, result)
		if result.Terminated {
			return fmt.Errorf("terminate operation by plugin hook.%v, detail-> %v", name, result.Msg)
		}
	}

	return nil
}
