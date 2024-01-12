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

package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/stretchr/testify/assert"
)

func TestCreateDefaultDomainDataSource(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	domainDataService := makeDomainDataSourceService(t, conf)
	res := domainDataService.CreateDefaultDomainDataSource(context.Background())
	assert.Nil(t, res)
}

func TestQueryDomainDataSource(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	domainDataService := makeDomainDataSourceService(t, conf)
	err = domainDataService.CreateDefaultDomainDataSource(context.Background())
	assert.Nil(t, err)
	queryRes := domainDataService.QueryDomainDataSource(context.Background(), &datamesh.QueryDomainDataSourceRequest{
		DatasourceId: common.DefaultDataSourceID,
	})
	assert.NotNil(t, queryRes)
	assert.Equal(t, queryRes.Data.Type, common.DomainDataSourceTypeLocalFS)
	assert.Equal(t, queryRes.Data.Info.Localfs.Path, common.DefaultDomainDataSourceLocalFSPath)
}

func makeDomainDataSourceService(t *testing.T, conf *config.DataMeshConfig) IDomainDataSourceService {
	return NewDomainDataSourceService(conf, makeMemConfigurationService(t))
}

func makeMemConfigurationService(t *testing.T) cmservice.IConfigurationService {
	backend, err := secretbackend.NewSecretBackendWith("mem", map[string]any{})
	assert.Nil(t, err)
	assert.NotNil(t, backend)
	configurationService, err := cmservice.NewConfigurationService(
		backend, false,
	)
	assert.Nil(t, err)
	assert.NotNil(t, configurationService)
	return configurationService
}
