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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func generateTestConfig(t *testing.T, rootDir string) string {
	content := fmt.Sprintf(`
rootDir: %s
`, filepath.Join(rootDir, "kuscia"))
	configPath := filepath.Join(rootDir, "agent.yaml")
	assert.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	return configPath
}

func TestLoadConfig(t *testing.T) {
	rootDir := t.TempDir()
	cfg, err := LoadAgentConfig(generateTestConfig(t, rootDir))
	assert.NoError(t, err)

	assert.Equal(t, cfg.LogsPath, filepath.Join(rootDir, "kuscia", "var/logs"))
}
