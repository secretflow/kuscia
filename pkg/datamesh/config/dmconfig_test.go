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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDataMeshConfig(t *testing.T) {
	config := NewDefaultDataMeshConfig()
	assert.Equal(t, 8070, int(config.HTTPPort))
	assert.Equal(t, 8071, int(config.GRPCPort))
	assert.Equal(t, 5, config.ConnectTimeOut)
	assert.Equal(t, 20, config.ReadTimeout)
	assert.Equal(t, 20, config.WriteTimeout)
	assert.Equal(t, 300, config.IdleTimeout)
	assert.False(t, config.DisableTLS)
}
