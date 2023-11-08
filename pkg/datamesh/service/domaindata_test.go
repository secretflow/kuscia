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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

var (
	dsID = GetDefaultDataSourceID()
)

func createTestDomainDataSource(t *testing.T, conf *config.DataMeshConfig) string {
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
		AccessDirectly: false,
	})
	assert.NotNil(t, res)

	exist := false
	for i := 0; i < 10; {
		if _, err := conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Get(context.Background(),
			dsID, metav1.GetOptions{}); err == nil {
			exist = true
			break
		}
		time.Sleep(time.Second)
		i++
	}
	assert.True(t, exist)
	return res.Data.DatasourceId
}

func TestCreateDomainData(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}

	mockDsID := createTestDomainDataSource(t, conf)
	domainDataService := NewDomainDataService(conf)
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: mockDsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	assert.NotNil(t, res)
	assert.True(t, res.Status.Code == 0)

	col[1].Type = "double"
	res = domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/d.csv",
		DatasourceId: mockDsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      col,
	})
	assert.NotNil(t, res)
	assert.True(t, res.Status.Code != 0)
}

func TestQueryDomainData(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	createTestDomainDataSource(t, conf)
	domainDataService := NewDomainDataService(conf)
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	res1 := domainDataService.QueryDomainData(context.Background(), &datamesh.QueryDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
	})
	assert.NotNil(t, res1)
}

func TestUpdateDomainData(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	createTestDomainDataSource(t, conf)
	domainDataService := NewDomainDataService(conf)
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	res1 := domainDataService.UpdateDomainData(context.Background(), &datamesh.UpdateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res1)
}

func TestDeleteDomainData(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	createTestDomainDataSource(t, conf)
	domainDataService := NewDomainDataService(conf)
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: dsID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	res1 := domainDataService.DeleteDomainData(context.Background(), &datamesh.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
	})
	assert.NotNil(t, res1)
}

func TestCheckCols(t *testing.T) {
	cols := []*v1alpha1.DataColumn{
		{
			Name:        "col1",
			Type:        "int32",
			Comment:     "",
			NotNullable: false,
		},
	}
	assert.NotNil(t, CheckColType(cols, "mysql"))
	assert.Nil(t, CheckColType(cols, "oss"))

	cols = []*v1alpha1.DataColumn{
		{
			Name:        "col1",
			Type:        "xxx",
			Comment:     "",
			NotNullable: false,
		},
	}
	assert.NotNil(t, CheckColType(cols, "oss"))
}
