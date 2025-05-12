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

func TestRemoveCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := rmCommand(cmdCtx)

	assert.Equal(t, "rm image [OPTIONS]", cmd.Use)
	assert.Equal(t, "Remove local one image by imageName or imageID", cmd.Short)

	// Check if cmd.Args is of type cobra.MinimumNArgs(1)
	assert.NotNil(t, cmd.Args)

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image rm 1111111")
	assert.Contains(t, example, "kuscia image rm my-image:latest")
}
