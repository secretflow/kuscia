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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/errdefs"
)

func TestStore(t *testing.T) {
	s := NewStore()

	container1 := &Container{Metadata: Metadata{ID: "111"}}
	container2 := &Container{Metadata: Metadata{ID: "222"}}

	assert.NoError(t, s.Add(container1))
	assert.NoError(t, s.Add(container2))
	_, err := s.Get("111")
	assert.NoError(t, err)
	s.Delete("111")
	_, err = s.Get("111")
	assert.ErrorIs(t, err, errdefs.ErrNotFound)
	containerList := s.List()
	assert.Equal(t, 1, len(containerList))
	assert.Equal(t, "222", containerList[0].ID)
}
