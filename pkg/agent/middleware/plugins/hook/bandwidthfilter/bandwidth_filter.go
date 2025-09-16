// Copyright 2025 Ant Group Co., Ltd.
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

package bandwidthfilter

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func Register() {
	plugin.Register(common.PluginNameBandwidthFilter, &bandwidthFilter{})
}

type bandwidthFilterConfig struct {
	// Whether to strip bandwidth resources when running in runk; defaults to true if not configured (nil).
	StripOnRunk *bool `json:"stripOnRunk" yaml:"stripOnRunk"`
}

type bandwidthFilter struct {
	initialized bool
	stripOnRunk bool
	isRunk      bool
}

// Type implements plugin.Plugin.
func (bf *bandwidthFilter) Type() string {
	return hook.PluginType
}

// Init implements plugin.Plugin.
// Only registers to the hook system when runtime == runk and stripOnRunk is set to true.
func (bf *bandwidthFilter) Init(ctx context.Context, deps *plugin.Dependencies, cfg *config.PluginCfg) error {
	// Default: On
	bf.stripOnRunk = true

	if cfg != nil {
		var c bandwidthFilterConfig
		if err := cfg.Config.Decode(&c); err == nil && c.StripOnRunk != nil {
			bf.stripOnRunk = *c.StripOnRunk
		}
	}

	rt := ""
	if deps != nil && deps.AgentConfig != nil {
		rt = strings.ToLower(strings.TrimSpace(deps.AgentConfig.Provider.Runtime))
	}
	bf.isRunk = rt == config.K8sRuntime

	nlog.Infof("[bandwidthfilter] Init: detected runtime=%q (raw=%q), stripOnRunk=%v",
		rt, deps.AgentConfig.Provider.Runtime, bf.stripOnRunk)

	// If the runtime is not runk, or the configuration disables it, the plugin will not be registered.
	if !bf.isRunk || !bf.stripOnRunk {
		nlog.Infof("Plugin %s NOT registered (runtime=%q, stripOnRunk=%v)",
			common.PluginNameBandwidthFilter, rt, bf.stripOnRunk)
		return nil
	}

	bf.initialized = true
	hook.Register(common.PluginNameBandwidthFilter, bf)
	nlog.Infof("Plugin %s registered (runtime=%q)", common.PluginNameBandwidthFilter, rt)
	return nil
}

// Only executed at the stage “before syncing to the external Kubernetes”.
func (bf *bandwidthFilter) CanExec(ctx hook.Context) bool {
	if !bf.initialized {
		return false
	}
	switch ctx.Point() {
	case hook.PointK8sProviderSyncPod:
		_, ok := ctx.(*hook.K8sProviderSyncPodContext)
		return ok
	default:
		return false
	}
}

// ExecHook: strips the bandwidth extended resource from the BkPod before it is dispatched.
func (bf *bandwidthFilter) ExecHook(ctx hook.Context) (*hook.Result, error) {
	syncCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
	if !ok || syncCtx.BkPod == nil {
		return &hook.Result{}, nil
	}

	scrub := func(rr *corev1.ResourceRequirements) {
		if rr == nil {
			return
		}
		delete(rr.Limits, common.ResourceBandwidth)
		delete(rr.Requests, common.ResourceBandwidth)
	}

	for i := range syncCtx.BkPod.Spec.Containers {
		scrub(&syncCtx.BkPod.Spec.Containers[i].Resources)
	}
	for i := range syncCtx.BkPod.Spec.InitContainers {
		scrub(&syncCtx.BkPod.Spec.InitContainers[i].Resources)
	}
	for i := range syncCtx.BkPod.Spec.EphemeralContainers {
		scrub(&syncCtx.BkPod.Spec.EphemeralContainers[i].Resources)
	}
	if syncCtx.BkPod.Spec.Overhead != nil {
		delete(syncCtx.BkPod.Spec.Overhead, common.ResourceBandwidth)
	}

	return &hook.Result{}, nil
}
