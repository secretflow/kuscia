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

package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
)

func TestConfigNode(t *testing.T) {
	agentCfg := config.DefaultAgentConfig()
	agentCfg.RootDir = t.TempDir()

	cfg := &GenericNodeConfig{
		Namespace:    "namespace1",
		NodeName:     "node1",
		NodeIP:       "1.1.1.1",
		APIVersion:   "0.2",
		AgentVersion: "0.3",
		PodsAuditor:  NewPodsAuditor(agentCfg),
		AgentConfig:  agentCfg,
	}

	np, err := NewNodeProvider(cfg)
	assert.NoError(t, err)
	assert.NoError(t, np.Ping(context.Background()))

	nmt := np.ConfigureNode(context.Background())
	assert.Equal(t, nmt.Name, cfg.NodeName)
	assert.Equal(t, nmt.Status.NodeInfo.KubeletVersion, cfg.AgentVersion)
	assert.Equal(t, nmt.Labels["kubernetes.io/apiVersion"], cfg.APIVersion)
	assert.Equal(t, nmt.Labels[common.LabelNodeNamespace], cfg.Namespace)

	assert.True(t, len(nmt.Status.Conditions) > 0)
}
