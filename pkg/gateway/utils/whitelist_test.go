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

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func generateTestFile(t *testing.T) string {
	content := `
items:
  -
    address: 192.168.1.1
    ports:
      - 80
      - 1080
`
	configPath := filepath.Join(t.TempDir(), "whitelist.yaml")
	assert.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	return configPath
}

func TestWhitelistCheck(t *testing.T) {
	checker, err := NewWhitelistChecker(generateTestFile(t))
	assert.NoError(t, err)
	assert.True(t, checker.Check("192.168.1.1", []uint32{80, 1080}))
	assert.False(t, checker.Check("192.168.1.2", []uint32{80}))
}
