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
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/stretchr/testify/assert"
)

var (
	dsID = GetDefaultDataSourceID()
)

func TestCreateDomainData(t *testing.T) {
	conf := config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
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
	res = domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/d.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
}

func TestQueryDomainData(t *testing.T) {

	conf := config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
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
	conf := config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
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
	conf := config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
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
	res1 := domainDataService.DeleteDomainData(context.Background(), &datamesh.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
	})
	assert.NotNil(t, res1)
}
