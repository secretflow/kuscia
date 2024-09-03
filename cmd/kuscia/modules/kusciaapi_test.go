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

package modules

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/common"
)

func Test_RunKusciaAPI(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	m, err := NewKusciaAPI(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}

func Test_RunKusciaAPIWithTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	config.KusciaAPI.HTTPPort = 8010
	config.KusciaAPI.GRPCPort = 8011
	config.KusciaAPI.HTTPInternalPort = 8012
	config.Protocol = common.TLS
	m, err := NewKusciaAPI(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}

func Test_RunKusciaAPIWithMTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	config.KusciaAPI.HTTPPort = 8020
	config.KusciaAPI.GRPCPort = 8021
	config.KusciaAPI.HTTPInternalPort = 8022
	config.Protocol = common.MTLS
	m, err := NewKusciaAPI(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}

func Test_RunKusciaAPIWithNOTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	config.KusciaAPI.HTTPPort = 8030
	config.KusciaAPI.GRPCPort = 8031
	config.KusciaAPI.HTTPInternalPort = 8032
	config.Protocol = common.NOTLS
	m, err := NewKusciaAPI(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}
