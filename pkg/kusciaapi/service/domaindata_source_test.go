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
	"github.com/stretchr/testify/assert"

	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func makeDomainDataSourceServiceConfig(t *testing.T) *config.KusciaAPIConfig {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &config.KusciaAPIConfig{
		DomainKey:    privateKey,
		KusciaClient: kusciafake.NewSimpleClientset(),
		RunMode:      common.RunModeLite,
		Initiator:    "alice",
		DomainID:     "alice",
	}
}

func TestCreateDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	res := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     "alice",
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeLocalFS,
		Info: &kusciaapi.DataSourceInfo{
			Localfs: &kusciaapi.LocalDataSourceInfo{
				Path: "./data",
			},
		},
	})
	assert.Equal(t, dataSourceID, res.Data.DatasourceId)
}

func TestUpdateDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	createRes := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     "alice",
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeMysql,
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root",
				Password: "passwd",
				Database: "db-name",
			},
		},
	})
	assert.Equal(t, dataSourceID, createRes.Data.DatasourceId)

	updateRes := dsService.UpdateDomainDataSource(context.Background(), &kusciaapi.UpdateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     "alice",
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeMysql,
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root-2",
				Password: "passwd-2",
				Database: "db-name-2",
			},
		},
	})
	assert.Equal(t, int32(0), updateRes.Status.Code)

	queryRes := dsService.QueryDomainDataSource(context.Background(), &kusciaapi.QueryDomainDataSourceRequest{
		DomainId:     "alice",
		DatasourceId: dataSourceID,
	})

	assert.Equal(t, int32(0), queryRes.Status.Code)
	assert.Equal(t, "root-2", queryRes.Data.Info.Database.User)
	assert.Equal(t, "passwd-2", queryRes.Data.Info.Database.Password)
	assert.Equal(t, "db-name-2", queryRes.Data.Info.Database.Database)
}

func TestDeleteDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)

	createRes := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     "alice",
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeMysql,
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root",
				Password: "passwd",
				Database: "db-name",
			},
		},
	})
	assert.Equal(t, dataSourceID, createRes.Data.DatasourceId)

	deleteRes := dsService.DeleteDomainDataSource(context.Background(), &kusciaapi.DeleteDomainDataSourceRequest{
		DomainId:     "alice",
		DatasourceId: dataSourceID,
	})
	assert.Equal(t, int32(0), deleteRes.Status.Code)

	queryRes := dsService.QueryDomainDataSource(context.Background(), &kusciaapi.QueryDomainDataSourceRequest{
		DomainId:     "alice",
		DatasourceId: dataSourceID,
	})
	assert.NotEqual(t, int32(0), queryRes.Status.Code)
}

func TestBatchQueryDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	createRes := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     "alice",
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeMysql,
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root",
				Password: "passwd",
				Database: "db-name",
			},
		},
	})
	assert.Equal(t, dataSourceID, createRes.Data.DatasourceId)

	batchQueryRes := dsService.BatchQueryDomainDataSource(context.Background(), &kusciaapi.BatchQueryDomainDataSourceRequest{
		Data: []*kusciaapi.QueryDomainDataSourceRequestData{
			{
				DomainId:     "alice",
				DatasourceId: dataSourceID,
			},
		},
	})
	assert.Equal(t, int32(0), batchQueryRes.Status.Code)
	assert.Equal(t, common.DomainDataSourceTypeMysql, batchQueryRes.Data.DatasourceList[0].Type)
}

func makeDomainDataSourceService(t *testing.T, conf *config.KusciaAPIConfig) IDomainDataSourceService {
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
