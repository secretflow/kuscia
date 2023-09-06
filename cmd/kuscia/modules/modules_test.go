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

package modules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/common"
)

func Test_EnsureCaKeyAndCert(t *testing.T) {
	rootDir := t.TempDir()
	_, _, err := EnsureCaKeyAndCert(&Dependencies{
		KusciaConfig: KusciaConfig{
			CAKeyFile: filepath.Join(rootDir, "ca.key"),
			CAFile:    filepath.Join(rootDir, "ca.crt"),
			DomainID:  "alice",
		},
	})
	assert.NoError(t, err)
	_, _, err = EnsureCaKeyAndCert(&Dependencies{
		KusciaConfig: KusciaConfig{
			CAKeyFile: filepath.Join(rootDir, "ca.key"),
			CAFile:    filepath.Join(rootDir, "ca.crt"),
			DomainID:  "alice",
		},
	})
	assert.NoError(t, err)
}

func Test_EnsureDomainKey(t *testing.T) {
	rootDir := t.TempDir()
	err := EnsureDomainKey(&Dependencies{
		KusciaConfig: KusciaConfig{
			DomainKeyFile: filepath.Join(rootDir, "domain.key"),
		},
	})
	assert.NoError(t, err)
}

func Test_RenderConfig(t *testing.T) {
	rootDir := t.TempDir()
	configPathTmpl := filepath.Join(rootDir, "config.tmpl")
	configPath := filepath.Join(rootDir, "config")
	file, _ := os.Create(configPathTmpl)
	file.WriteString(`{{.alice}}`)
	file.Close()
	err := common.RenderConfig(configPathTmpl, configPath, map[string]string{"alice": "bob"})
	assert.NoError(t, err)
}

func Test_EnsureDir(t *testing.T) {
	rootDir := t.TempDir()
	err := EnsureDir(&Dependencies{
		KusciaConfig: KusciaConfig{
			RootDir: rootDir,
		},
	})
	assert.NoError(t, err)
}
