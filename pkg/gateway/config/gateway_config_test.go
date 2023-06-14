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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
)

func TestLoadConfig(t *testing.T) {
	config := DefaultStaticGatewayConfig()
	err := config.CheckConfig()
	assert.Error(t, err)

	config.MasterConfig = &kusciaconfig.MasterConfig{
		APIServer: &kusciaconfig.APIServerConfig{
			KubeConfig: "your path of kube config",
			Endpoint:   "http://127.0.0.1:80",
		},
		KusciaStorage: &kusciaconfig.ServiceConfig{
			Endpoint:  "http://127.0.0.1:80",
			TLSConfig: nil,
		},
	}
	config.MasterConfig.APIServer = &kusciaconfig.APIServerConfig{
		KubeConfig: "your path of kube config",
		Endpoint:   "http://127.0.0.1:80",
	}
	err = config.CheckConfig()
	assert.NoError(t, err)
}
