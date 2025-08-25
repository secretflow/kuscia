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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

var (
	dsID = common.DefaultDataSourceID
)

func createTestDefaultDomainDataSource(t *testing.T, conf *config.DataMeshConfig) string {
	domainDataService := makeDomainDataSourceService(t, conf)
	err := domainDataService.CreateDefaultDomainDataSource(context.Background())
	assert.Nil(t, err)

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
	return common.DefaultDataSourceID
}

func TestCreateDomainData(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	mockDsID := createTestDefaultDomainDataSource(t, conf)
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
	assert.True(t, res.Status.Code == 0)
}

func TestQueryDomainData(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	createTestDefaultDomainDataSource(t, conf)
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
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	createTestDefaultDomainDataSource(t, conf)
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
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	createTestDefaultDomainDataSource(t, conf)
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
			Type:        "binary",
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

func TestConvert2UpdateReq(t *testing.T) {
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}

	createReq := &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "test-id",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: "ds-1",
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns:    col,
		Vendor:     "test-vendor",
		FileFormat: v1alpha1.FileFormat_UNKNOWN,
	}

	updateReq := convert2UpdateReq(createReq)

	assert.Equal(t, createReq.Header, updateReq.Header)
	assert.Equal(t, createReq.DomaindataId, updateReq.DomaindataId)
	assert.Equal(t, createReq.Name, updateReq.Name)
	assert.Equal(t, createReq.Type, updateReq.Type)
	assert.Equal(t, createReq.RelativeUri, updateReq.RelativeUri)
	assert.Equal(t, createReq.DatasourceId, updateReq.DatasourceId)
	assert.Equal(t, createReq.Attributes, updateReq.Attributes)
	assert.Equal(t, createReq.Partition, updateReq.Partition)
	assert.Equal(t, createReq.Columns, updateReq.Columns)
	assert.Equal(t, createReq.Vendor, updateReq.Vendor)
	assert.Equal(t, createReq.FileFormat, updateReq.FileFormat)
}

func TestConvert2CreateResp(t *testing.T) {
	updateResp := &datamesh.UpdateDomainDataResponse{
		Status: &v1alpha1.Status{
			Code:    0,
			Message: "success",
		},
	}
	domainDataID := "test-id"

	createResp := convert2CreateResp(updateResp, domainDataID)

	assert.Equal(t, updateResp.Status, createResp.Status)
	assert.Equal(t, domainDataID, createResp.Data.DomaindataId)
}
