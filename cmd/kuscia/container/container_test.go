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

package container

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewContainerCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewContainerCommand(ctx)

	assert.Equal(t, "container", cmd.Use)
	assert.Equal(t, "Manage containers", cmd.Short)

	// Test flags
	storeFlag := cmd.PersistentFlags().Lookup("store")
	assert.NotNil(t, storeFlag)
	assert.Equal(t, "kuscia image storage directory", storeFlag.Usage)

	runtimeFlag := cmd.PersistentFlags().Lookup("runtime")
	assert.NotNil(t, runtimeFlag)
	assert.Equal(t, "kuscia runtime type: runp/runc", runtimeFlag.Usage)

	// Test command execution (no-op for now)
	cmd.PersistentPreRun(&cobra.Command{}, []string{})
}
