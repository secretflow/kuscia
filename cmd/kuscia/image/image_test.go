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

package image

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestNewImageCommand(t *testing.T) {
	t.Run("CommandStructure", func(t *testing.T) {
		ctx := context.Background()
		cmd := NewImageCommand(ctx)
		
		assert.Equal(t, "image", cmd.Use)
		assert.Equal(t, "Manage images", cmd.Short)
		assert.Equal(t, []string{"images"}, cmd.Aliases)
		assert.True(t, cmd.SilenceUsage)
	})

	t.Run("PersistentFlags", func(t *testing.T) {
		ctx := context.Background()
		cmd := NewImageCommand(ctx)
		
		
		storeFlag := cmd.PersistentFlags().Lookup("store")
		require.NotNil(t, storeFlag)
		assert.Equal(t, config.DefaultImageStoreDir(), storeFlag.DefValue)
		
		
		runtimeFlag := cmd.PersistentFlags().Lookup("runtime")
		require.NotNil(t, runtimeFlag)
		assert.Empty(t, runtimeFlag.DefValue)
	})
}