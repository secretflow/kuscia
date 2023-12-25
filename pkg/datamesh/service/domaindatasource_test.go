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
	"testing"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/stretchr/testify/assert"
)

func TestCreateDomainDataSource(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	domainDataService := NewDomainDataSourceService(conf)
	res := domainDataService.CreateDomainDataSource(context.Background(), &datamesh.CreateDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
		Name:         "default-datasource",
		Type:         "localfs",
		Info: &datamesh.DataSourceInfo{
			Localfs: &datamesh.LocalDataSourceInfo{
				Path: "./data",
			},
			Oss: nil,
		},
	})
	assert.NotNil(t, res)
}

func TestQueryDomainDataSource(t *testing.T) {

	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	domainDataService := NewDomainDataSourceService(conf)
	res := domainDataService.CreateDomainDataSource(context.Background(), &datamesh.CreateDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
		Name:         "default-datasource",
		Type:         "localfs",
		Info: &datamesh.DataSourceInfo{
			Localfs: &datamesh.LocalDataSourceInfo{
				Path: "./data",
			},
			Oss: nil,
		},
	})
	assert.NotNil(t, res)
	queryRes := domainDataService.QueryDomainDataSource(context.Background(), &datamesh.QueryDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
	})
	assert.NotNil(t, queryRes)
	assert.Equal(t, queryRes.Data.Type, "localfs")
	assert.Equal(t, queryRes.Data.Info.Localfs.Path, "./data")
}

func TestUpdateDomainDataSource(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	domainDataService := NewDomainDataSourceService(conf)
	res := domainDataService.CreateDomainDataSource(context.Background(), &datamesh.CreateDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
		Name:         "default-datasource",
		Type:         "mysql",
		Info: &datamesh.DataSourceInfo{
			Database: &datamesh.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root",
				Password: "passwd",
			},
		},
	})
	assert.NotNil(t, res)

	updateRes := domainDataService.UpdateDomainDataSource(context.Background(), &datamesh.UpdateDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
		Type:         "mysql",
		Info: &datamesh.DataSourceInfo{
			Database: &datamesh.DatabaseDataSourceInfo{
				Endpoint: "127.0.0.1:3306",
				User:     "root1",
				Password: "passwd1",
			},
		},
	})
	assert.NotNil(t, updateRes)

	queryRes := domainDataService.QueryDomainDataSource(context.Background(), &datamesh.QueryDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
	})
	assert.NotNil(t, queryRes)
	assert.Equal(t, "mysql", queryRes.Data.Type)
	assert.Equal(t, "root1", queryRes.Data.Info.Database.User)
	assert.Equal(t, "passwd1", queryRes.Data.Info.Database.Password)
	assert.Equal(t, false, queryRes.Data.AccessDirectly)
}

func TestDeleteDomainDataSource(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	domainDataService := NewDomainDataSourceService(conf)
	res := domainDataService.DeleteDomainDataSource(context.Background(), &datamesh.DeleteDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: dsID,
	})
	assert.NotNil(t, res)
}
