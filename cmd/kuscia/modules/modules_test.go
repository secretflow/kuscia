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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"

	"github.com/secretflow/kuscia/pkg/utils/common"
)

func Test_LoadCaDomainKeyAndCert(t *testing.T) {
	rootDir := t.TempDir()
	de := &Dependencies{
		KusciaConfig: confloader.KusciaConfig{
			CAKeyFile:     filepath.Join(rootDir, "ca.key"),
			CACertFile:    filepath.Join(rootDir, "ca.crt"),
			DomainKeyFile: filepath.Join(rootDir, "domain.key"),
			DomainID:      "alice",
		},
	}
	err := de.LoadCaDomainKeyAndCert()
	assert.NotEmpty(t, err)
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
		KusciaConfig: confloader.KusciaConfig{
			RootDir: rootDir,
		},
	})
	assert.NoError(t, err)
}

func Test_LoadKusciaConfig(t *testing.T) {
	config := &confloader.KusciaConfig{}
	content := fmt.Sprintf(`
rootDir: /home/kuscia
domainID: kuscia
caKeyFile: var/tmp/ca.key
caFile: var/tmp/ca.crt
domainKeyFile: var/tmp/domain.key
master:
  endpoint: http://127.0.0.1:1080
  tls:
    certFile: var/tmp/client-admin.crt
    keyFile: var/tmp/client-admin.key
    caFile: var/tmp/server-ca.crt
  apiserver:
    kubeconfigFile: etc/kubeconfig
    endpoint:  http://127.0.0.1:1080
agent:
  allowPrivileged: false
externalTLS:
  certFile: var/tmp/external_tls.crt
  keyFile: var/tmp/external_tls.key
`)
	err := yaml.Unmarshal([]byte(content), config)
	assert.NoError(t, err)
}
