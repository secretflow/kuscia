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

package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestTagCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := tagCommand(cmdCtx)

	assert.Equal(t, "tag SOURCE_IMAGE[:TAG] TARGET_IMAGE[:TAG]", cmd.Use)
	assert.Equal(t, "Create a tag TARGET_IMAGE that refers to SOURCE_IMAGE", cmd.Short)

	// Check if cmd.Args is of type cobra.ExactArgs(2)
	assert.NotNil(t, cmd.Args)

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image tag secretflow/secretflow:v1 registry.mycompany.com/secretflow/secretflow:v1")
}
