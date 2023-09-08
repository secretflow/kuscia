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

//nolint:dulp
package utils

import (
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGetInitConfMaster(t *testing.T) {
	conf := GetInitConfig("", "", RunModeMaster)
	assert.Equal(t, conf.RootDir, defaultRootDir, "GetInitConfig test failed")
	assert.Equal(t, conf.DomainID, defaultDomainID, "GetInitConfig test failed")
	assert.Equal(t, conf.ApiserverEndpoint, defaultEndpoint, "GetInitConfig test failed")

	assert.Equal(t, conf.KubeconfigFile, filepath.Join(defaultRootDir, "etc/kubeconfig"), "GetInitConfig test failed")
	assert.Equal(t, conf.KusciaKubeConfig, filepath.Join(defaultRootDir, "etc/kuscia.kubeconfig"), "GetInitConfig test failed")
	assert.Equal(t, conf.CAKeyFile, filepath.Join(defaultRootDir, modules.CertPrefix, "ca.key"), "GetInitConfig test failed")
	assert.Equal(t, conf.CAFile, filepath.Join(defaultRootDir, modules.CertPrefix, "ca.crt"), "GetInitConfig test failed")
	assert.Equal(t, conf.DomainKeyFile, filepath.Join(defaultRootDir, modules.CertPrefix, "domain.key"), "GetInitConfig test failed")
	assert.Equal(t, conf.InterConnSchedulerPort, defaultInterConnSchedulerPort, "GetInitConfig test failed")

}
func TestGetInitConfLite(t *testing.T) {
	generateTestConfig()
	conf := GetInitConfig("", "", RunModeLite)
	assert.Equal(t, conf.RootDir, defaultRootDir, "GetInitConfig test failed")
	assert.Equal(t, conf.DomainID, defaultDomainID, "GetInitConfig test failed")
	assert.Equal(t, conf.ApiserverEndpoint, defaultEndpointForLite, "GetInitConfig test failed")
	assert.Equal(t, conf.TransportPort, config.DefaultTransConfig().HTTPConfig.Port, "GetInitConfig test failed")
}
func TestGetInitConfAutonomy(t *testing.T) {
	generateTestConfig()
	conf := GetInitConfig("", "", RunModeAutonomy)
	assert.Equal(t, conf.RootDir, defaultRootDir, "GetInitConfig test failed")
	assert.Equal(t, conf.DomainID, defaultDomainID, "GetInitConfig test failed")
	assert.Equal(t, conf.ApiserverEndpoint, defaultEndpoint, "GetInitConfig test failed")

	assert.Equal(t, conf.KubeconfigFile, filepath.Join(defaultRootDir, "etc/kubeconfig"), "GetInitConfig test failed")
	assert.Equal(t, conf.KusciaKubeConfig, filepath.Join(defaultRootDir, "etc/kuscia.kubeconfig"), "GetInitConfig test failed")
	assert.Equal(t, conf.CAKeyFile, filepath.Join(defaultRootDir, modules.CertPrefix, "ca.key"), "GetInitConfig test failed")
	assert.Equal(t, conf.CAFile, filepath.Join(defaultRootDir, modules.CertPrefix, "ca.crt"), "GetInitConfig test failed")
	assert.Equal(t, conf.DomainKeyFile, filepath.Join(defaultRootDir, modules.CertPrefix, "domain.key"), "GetInitConfig test failed")
	assert.Equal(t, conf.InterConnSchedulerPort, defaultInterConnSchedulerPort, "GetInitConfig test failed")

	assert.Equal(t, conf.TransportPort, config.DefaultTransConfig().HTTPConfig.Port, "GetInitConfig test failed")
}

func generateTestConfig() {
	transCon := config.DefaultTransConfig()
	os.MkdirAll(filepath.Join(defaultRootDir, "etc/conf/transport/"), 0755)
	writeYaml(filepath.Join(defaultRootDir, "etc/conf/transport/transport.yaml"), transCon)

}

func writeYaml(path string, serverC *config.TransConfig) error {
	data, err := yaml.Marshal(serverC)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, data, 0777)
	if err != nil {
		return err
	}
	return nil
}
