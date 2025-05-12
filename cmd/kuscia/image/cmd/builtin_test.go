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

package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestBuiltinCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := builtinCommand(cmdCtx)

	assert.Equal(t, "builtin [OPTIONS]", cmd.Use)
	assert.Equal(t, "Load a built-in image", cmd.Short)
	assert.NotNil(t, cmd.Flags().Lookup("manifest"))

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image builtin secretflow:latest")
}
