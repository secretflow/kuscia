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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var mockDomainId = "alice"

func makeDomainDataSourceServiceConfig(t *testing.T) *config.KusciaAPIConfig {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &config.KusciaAPIConfig{
		DomainKey:    privateKey,
		KusciaClient: kusciafake.NewSimpleClientset(MakeMockDomain(mockDomainId)),
		RunMode:      common.RunModeLite,
		Initiator:    "alice",
		DomainID:     mockDomainId,
	}
}

func TestCreateDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	res := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomainId,
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

func TestCreateDomainDataSource_InfoKeyNotExists(t *testing.T) {
	dataSourceID := "ds-1"
	makeInfoKey := "test"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	res := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomainId,
		DatasourceId: dataSourceID,
		Type:         common.DomainDataSourceTypeLocalFS,
		Info: &kusciaapi.DataSourceInfo{
			Localfs: &kusciaapi.LocalDataSourceInfo{
				Path: "./data",
			},
		},
		InfoKey: &makeInfoKey,
	})

	assert.EqualValues(t, res.Status.Code, errorcode.ErrCreateDomainDataSource)
}

func TestUpdateDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	createRes := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomainId,
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
		DomainId:     mockDomainId,
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
		DomainId:     mockDomainId,
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
		DomainId:     mockDomainId,
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
		DomainId:     mockDomainId,
		DatasourceId: dataSourceID,
	})
	assert.Equal(t, int32(0), deleteRes.Status.Code)

	queryRes := dsService.QueryDomainDataSource(context.Background(), &kusciaapi.QueryDomainDataSourceRequest{
		DomainId:     mockDomainId,
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
		DomainId:     mockDomainId,
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
				DomainId:     mockDomainId,
				DatasourceId: dataSourceID,
			},
		},
	})
	assert.Equal(t, int32(0), batchQueryRes.Status.Code)
	assert.Equal(t, common.DomainDataSourceTypeMysql, batchQueryRes.Data.DatasourceList[0].Type)
}

func TestListDomainDataSource(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	createRes := dsService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomainId,
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
	res := dsService.ListDomainDataSource(context.Background(), &kusciaapi.ListDomainDataSourceRequest{
		DomainId: mockDomainId,
	})
	assert.Equal(t, int32(0), res.Status.Code)
	assert.Equal(t, common.DomainDataSourceTypeMysql, res.Data.DatasourceList[0].Type)
}

func TestListDomainDataSource_NotExist(t *testing.T) {
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	res := dsService.ListDomainDataSource(context.Background(), &kusciaapi.ListDomainDataSourceRequest{
		DomainId: mockDomainId,
	})
	assert.Equal(t, int32(0), res.Status.Code)
	assert.Equal(t, 0, len(res.GetData().DatasourceList))
}

func TestListDomainDataSource_DomainNotExist(t *testing.T) {
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	res := dsService.ListDomainDataSource(context.Background(), &kusciaapi.ListDomainDataSourceRequest{
		DomainId: "mock-domain-id",
	})
	assert.Equal(t, int32(errorcode.ErrListDomainDataSource), res.Status.Code)
}

func TestListDomainDataSource_InfoErr(t *testing.T) {
	dataSourceID := "ds-1"
	conf := makeDomainDataSourceServiceConfig(t)
	dsService := makeDomainDataSourceService(t, conf)
	dataSource := &v1alpha1.DomainDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataSourceID,
			Namespace: mockDomainId,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Type: "oss",
		},
	}
	_, err := conf.KusciaClient.KusciaV1alpha1().DomainDataSources(mockDomainId).Create(context.Background(), dataSource, metav1.CreateOptions{})
	assert.NoError(t, err)
	res := dsService.ListDomainDataSource(context.Background(), &kusciaapi.ListDomainDataSourceRequest{
		DomainId: mockDomainId,
	})
	assert.Equal(t, int32(errorcode.ErrListDomainDataSource), res.Status.Code)
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
