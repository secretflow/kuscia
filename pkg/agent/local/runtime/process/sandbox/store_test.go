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

package sandbox

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestStore(t *testing.T) {
	rootDir := t.TempDir()
	s := NewStore(rootDir)

	sandbox1, err := NewSandbox(&runtime.PodSandboxConfig{
		Metadata: &runtime.PodSandboxMetadata{
			Name:      "aaa",
			Uid:       "111",
			Namespace: "test-namespace",
		},
	}, "127.0.0.1", rootDir)
	assert.NoError(t, err)

	sandbox2, err := NewSandbox(&runtime.PodSandboxConfig{
		Metadata: &runtime.PodSandboxMetadata{
			Name:      "bbb",
			Uid:       "222",
			Namespace: "test-namespace",
		},
	}, "127.0.0.1", rootDir)
	assert.NoError(t, err)

	sandboxID := sandbox1.ID
	sandboxRootDir := sandbox1.Bundle.GetRootDirectory()

	assert.NoError(t, s.Add(sandbox1))
	assert.NoError(t, s.Add(sandbox2))
	_, ok := s.Get(sandboxID)
	assert.True(t, ok)
	s.Delete(sandboxID)
	_, ok = s.Get(sandboxID)
	assert.False(t, ok)
	sandboxList := s.List()
	assert.Equal(t, 1, len(sandboxList))

	s.cleanupInterval = 10 * time.Millisecond
	s.sandboxProtectionTime = 0
	s.Monitor()
	time.Sleep(1 * time.Second)
	_, err = os.Stat(sandboxRootDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
