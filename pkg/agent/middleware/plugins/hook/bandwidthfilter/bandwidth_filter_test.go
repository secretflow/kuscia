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
	"testing"

	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	y3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/common"
)

func qty(s string) resource.Quantity {
	return resource.MustParse(s)
}

func makePodWithBW() *corev1.Pod {
	bw := common.ResourceBandwidth

	return &corev1.Pod{
		Spec: corev1.PodSpec{
			Overhead: corev1.ResourceList{
				bw: qty("100"),
			},
			InitContainers: []corev1.Container{
				{
					Name: "init",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							bw: qty("200"),
						},
						Requests: corev1.ResourceList{
							bw: qty("100"),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "c1",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							bw:       qty("2k"),
							"cpu":    qty("500m"),
							"memory": qty("256Mi"),
						},
						Requests: corev1.ResourceList{
							bw:       qty("1k"),
							"cpu":    qty("250m"),
							"memory": qty("128Mi"),
						},
					},
				},
			},
			EphemeralContainers: []corev1.EphemeralContainer{
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name: "e1",
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								bw: qty("1500"),
							},
							Requests: corev1.ResourceList{
								bw: qty("1000"),
							},
						},
					},
				},
			},
		},
	}
}

func makeSyncCtx(pod *corev1.Pod) *hook.K8sProviderSyncPodContext {
	return &hook.K8sProviderSyncPodContext{
		Pod:   pod,
		BkPod: pod,
	}
}

func makePluginCfgYAML(yamlStr string) *config.PluginCfg {
	var node y3.Node
	_ = y3.Unmarshal([]byte(yamlStr), &node)
	return &config.PluginCfg{
		Config: node,
	}
}

func TestExecHook_StripOnRunk(t *testing.T) {
	// runtime=runk and stripOnRunk=true
	agentCfg := &config.AgentConfig{}
	agentCfg.Provider.Runtime = "runk"

	p := &bandwidthFilter{}
	deps := &pluginDeps{AgentConfig: agentCfg}

	if err := p.Init(context.Background(), deps.ToDeps(), makePluginCfgYAML("stripOnRunk: true\n")); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	pod := makePodWithBW()
	ctx := makeSyncCtx(pod)

	if !p.CanExec(ctx) {
		t.Fatal("plugin should be executable under runk with strip enabled")
	}
	if _, err := p.ExecHook(ctx); err != nil {
		t.Fatalf("ExecHook failed: %v", err)
	}

	bw := common.ResourceBandwidth
	// containers
	ctr := ctx.BkPod.Spec.Containers[0]
	if _, ok := ctr.Resources.Limits[bw]; ok {
		t.Fatalf("container limits bandwidth should be stripped")
	}
	if _, ok := ctr.Resources.Requests[bw]; ok {
		t.Fatalf("container requests bandwidth should be stripped")
	}
	// init container
	ini := ctx.BkPod.Spec.InitContainers[0]
	if _, ok := ini.Resources.Limits[bw]; ok {
		t.Fatalf("init container limits bandwidth should be stripped")
	}
	if _, ok := ini.Resources.Requests[bw]; ok {
		t.Fatalf("init container requests bandwidth should be stripped")
	}
	// ephemeral
	eph := ctx.BkPod.Spec.EphemeralContainers[0].Resources
	if _, ok := eph.Limits[bw]; ok {
		t.Fatalf("ephemeral container limits bandwidth should be stripped")
	}
	if _, ok := eph.Requests[bw]; ok {
		t.Fatalf("ephemeral container requests bandwidth should be stripped")
	}
	// overhead
	if _, ok := ctx.BkPod.Spec.Overhead[bw]; ok {
		t.Fatalf("pod overhead bandwidth should be stripped")
	}

	if ctr.Resources.Limits.Cpu().Cmp(qty("500m")) != 0 ||
		ctr.Resources.Requests.Cpu().Cmp(qty("250m")) != 0 {
		t.Fatalf("cpu resources should be kept")
	}
}

func TestExecHook_NotRunk_NoStrip(t *testing.T) {
	// runtime!=runk
	agentCfg := &config.AgentConfig{}
	agentCfg.Provider.Runtime = "containerd"

	p := &bandwidthFilter{}
	deps := &pluginDeps{AgentConfig: agentCfg}

	if err := p.Init(context.Background(), deps.ToDeps(), makePluginCfgYAML("stripOnRunk: true\n")); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	pod := makePodWithBW()
	ctx := makeSyncCtx(pod)

	// can't exec
	if p.CanExec(ctx) {
		t.Fatalf("plugin should NOT be executable for non-runk runtime")
	}

	bw := common.ResourceBandwidth
	ctr := ctx.BkPod.Spec.Containers[0]
	if _, ok := ctr.Resources.Limits[bw]; !ok {
		t.Fatalf("container limits bandwidth should be kept for non-runk runtime")
	}
	if _, ok := ctr.Resources.Requests[bw]; !ok {
		t.Fatalf("container requests bandwidth should be kept for non-runk runtime")
	}
}

func TestExecHook_Runk_StripDisabled(t *testing.T) {
	// runtime=runk，but stripOnRunk=false
	agentCfg := &config.AgentConfig{}
	agentCfg.Provider.Runtime = "runk"

	p := &bandwidthFilter{}
	deps := &pluginDeps{AgentConfig: agentCfg}

	if err := p.Init(context.Background(), deps.ToDeps(), makePluginCfgYAML("stripOnRunk: false\n")); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	pod := makePodWithBW()
	ctx := makeSyncCtx(pod)

	if p.CanExec(ctx) {
		t.Fatalf("plugin should NOT be executable when strip is disabled")
	}

	bw := common.ResourceBandwidth
	ctr := ctx.BkPod.Spec.Containers[0]
	if _, ok := ctr.Resources.Limits[bw]; !ok {
		t.Fatalf("container limits bandwidth should be kept when strip is disabled")
	}
	if _, ok := ctr.Resources.Requests[bw]; !ok {
		t.Fatalf("container requests bandwidth should be kept when strip is disabled")
	}
}

type pluginDeps struct {
	AgentConfig *config.AgentConfig
}

func (d *pluginDeps) ToDeps() *plugin.Dependencies {
	return &plugin.Dependencies{
		AgentConfig: d.AgentConfig,
	}
}
