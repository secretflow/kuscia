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

package diagnose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDiagnoseCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewDiagnoseCommand(ctx)

	assert.Equal(t, "diagnose", cmd.Use)
	assert.Equal(t, "Diagnose means pre-test Kuscia", cmd.Short)

	subCommands := cmd.Commands()
	assert.Len(t, subCommands, 3)
}

func TestNewCDRCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewCDRCommand(ctx)

	assert.Equal(t, "cdr", cmd.Use)
	assert.Equal(t, "Diagnose the effectiveness of domain route", cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("speed"))
	assert.NotNil(t, flags.Lookup("rtt"))
	assert.NotNil(t, flags.Lookup("proxy-timeout"))
	assert.NotNil(t, flags.Lookup("size"))
	assert.NotNil(t, flags.Lookup("buffer"))
}

func TestNewNetworkCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewNetworkCommand(ctx)

	assert.Equal(t, "network", cmd.Use)
	assert.Equal(t, "Diagnose the status of network between domains", cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("speed"))
	assert.NotNil(t, flags.Lookup("rtt"))
	assert.NotNil(t, flags.Lookup("proxy-timeout"))
	assert.NotNil(t, flags.Lookup("size"))
	assert.NotNil(t, flags.Lookup("buffer"))
	assert.NotNil(t, flags.Lookup("bidirection"))
}
