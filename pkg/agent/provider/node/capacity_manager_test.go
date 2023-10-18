// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestNewGenericNodeProvider(t *testing.T) {
	nonEmptyCfg := &config.CapacityCfg{
		CPU:     "1",
		Memory:  "1000000000",
		Pods:    "100",
		Storage: "100G",
	}
	emptyCfg := &config.CapacityCfg{}

	tests := []struct {
		localCapacity bool
		cfg           *config.CapacityCfg
		hasErr        bool
		useCfg        bool
	}{
		{
			localCapacity: true,
			cfg:           nonEmptyCfg,
			useCfg:        true,
		},
		{
			localCapacity: true,
			cfg:           emptyCfg,
			useCfg:        false,
		},
		{
			localCapacity: false,
			cfg:           nonEmptyCfg,
			useCfg:        true,
		},
		{
			localCapacity: false,
			cfg:           emptyCfg,
			hasErr:        true,
		},
	}

	rootDir := t.TempDir()

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			cp, err := NewCapacityManager(tt.cfg, rootDir, tt.localCapacity)
			if tt.hasErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.True(t, cp.cpuAvailable.Equal(cp.cpuTotal))
			assert.True(t, cp.podAvailable.Equal(cp.podTotal))
			if tt.useCfg {
				assert.True(t, cp.storageAvailable.Equal(cp.storageTotal))
				assert.True(t, cp.memAvailable.Equal(cp.memTotal) || cp.memAvailable.Cmp(cp.memTotal) < 0)
				assert.Equal(t, tt.cfg.CPU, cp.cpuAvailable.String())
				assert.Equal(t, tt.cfg.Storage, cp.storageAvailable.String())
				assert.Equal(t, tt.cfg.Pods, cp.podAvailable.String())
			}
		})
	}
}
