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

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPortAllocator(t *testing.T) {
	a := NewPortAllocator(1000, 2000)
	assert.NotNil(t, a)

	alloc, ok := a.(*randomPortAllocator)
	assert.True(t, ok)
	assert.NotNil(t, alloc)
	assert.Equal(t, 1000, alloc.minPort)
	assert.Equal(t, 1000, alloc.nextCheckPort)
	assert.Equal(t, 2000, alloc.maxPort)
}

func TestPortAlloc(t *testing.T) {
	a := NewPortAllocator(1000, 2000)
	assert.NotNil(t, a)

	port, err := a.Next()
	assert.NoError(t, err)
	assert.NotEqual(t, -1, int(port))
}
