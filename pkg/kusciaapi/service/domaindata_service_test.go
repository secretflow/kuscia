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
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/common"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	dsID     = common.DefaultDataSourceID
	domainId = "domain-data-unit-test-namespace"
)

func TestCreateDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	res = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/d.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)
}

func TestCreateDomainDataWithVendor(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}

	mustomVendor := "mustom-vendor"

	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
		Vendor:  mustomVendor,
	})
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	res1 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainId,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, mustomVendor, res1.Data.Vendor)
}

func TestQueryDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	res1 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainId,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, common.DefaultDomainDataVendor, res1.Data.Vendor)
}

func TestUpdateDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	res1 := domainDataService.UpdateDomainData(context.Background(), &kusciaapi.UpdateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)
}

func TestUpdateDomainDataWithVendor(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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

	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	mustomVendor := "mustom-vendor"
	res1 := domainDataService.UpdateDomainData(context.Background(), &kusciaapi.UpdateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
		Vendor:       mustomVendor,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)

	res2 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainId,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, mustomVendor, res2.Data.Vendor)

}

func TestDeleteDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	res1 := domainDataService.DeleteDomainData(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainId,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)
}

func TestBatchQueryDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	res1 := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
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
	datas := make([]*kusciaapi.QueryDomainDataRequestData, 2)
	datas[0] = &kusciaapi.QueryDomainDataRequestData{
		DomainId:     domainId,
		DomaindataId: res.Data.DomaindataId,
	}
	datas[1] = &kusciaapi.QueryDomainDataRequestData{
		DomainId:     domainId,
		DomaindataId: res1.Data.DomaindataId,
	}
	res2 := domainDataService.BatchQueryDomainData(context.Background(), &kusciaapi.BatchQueryDomainDataRequest{
		Header: nil,
		Data:   datas})
	assert.NotNil(t, res2)
}

func TestListDomainData(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: kusciafake.NewSimpleClientset(),
	}
	domainDataService := NewDomainDataService(conf)
	domainService := NewDomainService(conf)

	domainRes := domainService.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: domainId,
	})

	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	_ = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns:    col,
		FileFormat: v1alpha1.FileFormat_CSV,
	})
	_ = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainId,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns:    col,
		FileFormat: v1alpha1.FileFormat_CSV,
	})
	res2 := domainDataService.ListDomainData(context.Background(), &kusciaapi.ListDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.ListDomainDataRequestData{
			DomainId:         domainId,
			DomaindataType:   "table",
			DomaindataVendor: "manual",
		},
	},
	)
	assert.NotNil(t, res2)
}
