// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lite

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func newTestConfigFile(t *testing.T) string {
	root := t.TempDir()
	keyDataEncoded, err := tls.GenerateKeyData()
	assert.NoError(t, err, "generate key data error")
	path := filepath.Join(root, "config.yaml")
	content := fmt.Sprintf(`
mode: lite
rootDir: %s
domainID: alice
domainKeyData: %s
logLevel: INFO
liteDeployToken: cTWuN4TdHgAtnBiJ2bbdDHv6GMFMX5Y5
masterEndpoint: https://localhost:1080
runtime: runk
runk:
  namespace: test
  dnsServers:
  kubeconfigFile:
capacity:
  cpu: #4
  memory: #8Gi
  pods: #500
  storage: #100Gi
agent:
  node:
    enableNodeReuse: true
  capacity:
    cpu: 4
    memory: 4Gi
  provider:
    runtime: runk
    k8s:
      kubeconfigFile: /home/kuscia/etc/certs/k3s.yaml
      namespace: default
  allowPrivileged: true
  plugins:
    - name: config-render
`, root, keyDataEncoded)
	assert.NoError(t, os.WriteFile(path, []byte(content), 0644))

	transportDir := filepath.Join(root, "etc/conf/transport/")
	assert.NoError(t, os.MkdirAll(transportDir, 0755))
	transportFile := filepath.Join(transportDir, "transport.yaml")
	transportFileContent := `
msqConfig:
  SessionExpireSeconds: 600
  DeadSessionIDExpireSeconds: 1800
  TotalByteSizeLimit: 17179869184 # 16GB
  PerSessionByteSizeLimit: 62914560 # 60MB
httpConfig:
  port: 8081
  ReadTimeout: 300 # seconds
  WriteTimeout: 300 # seconds
  IdleTimeout: 60 # seconds
  ReqBodyMaxSize: 134217728 # 128MB
`
	assert.NoError(t, os.WriteFile(transportFile, []byte(transportFileContent), 0644))

	return path
}

func TestRenderInitConfig(t *testing.T) {
	configFile := newTestConfigFile(t)
	d := confloader.ReadConfig(configFile, "lite")
	assert.Equal(t, "runk", d.Agent.Provider.Runtime)
}
