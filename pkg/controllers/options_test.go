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
// limitations under the License.package controllers

package controllers

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_Options(t *testing.T) {
	t.Parallel()
	op := NewOptions()
	err := op.Validate()
	assert.NoError(t, err)
	op.HealthCheckPort = -1
	err = op.Validate()
	assert.True(t, err != nil)
	op.HealthCheckPort = 0
	op.Burst = -1
	err = op.Validate()
	assert.True(t, err != nil)
	op.Burst = 0
	op.QPS = -1
	err = op.Validate()
	assert.True(t, err != nil)
	op.QPS = 0

	cmd := &cobra.Command{}
	op.AddFlags(cmd.Flags())
}
