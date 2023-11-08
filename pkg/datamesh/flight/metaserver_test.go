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

package flight

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

var (
	dsID = service.GetDefaultDataSourceID()
)

type MockDataServer struct {
}

func mockFlightInfo(domainDataID string) *flight.FlightInfo {
	info := &flight.FlightInfo{
		Schema:           nil,
		FlightDescriptor: nil,
		Endpoint: []*flight.FlightEndpoint{
			{
				Ticket: &flight.Ticket{
					Ticket: []byte(fmt.Sprintf("%s-handler", domainDataID)),
				},
				Location: []*flight.Location{
					{
						Uri: "dataproxy.DomainDataUnitTestNamespace.svc",
					},
				},
				ExpirationTime: nil,
			},
		},
	}
	return info
}

func (m *MockDataServer) GetFlightInfoDataMeshQuery(ctx context.Context, query *datamesh.CommandDataMeshQuery) (*flight.FlightInfo, error) {
	return mockFlightInfo(query.Domaindata.DomaindataId), nil
}

func (m *MockDataServer) GetFlightInfoDataMeshUpdate(ctx context.Context, query *datamesh.CommandDataMeshUpdate) (*flight.FlightInfo, error) {
	return mockFlightInfo("test"), nil
}

func createTestDomainDataSource(t *testing.T, conf *config.DataMeshConfig) string {
	domainDataService := service.NewDomainDataSourceService(conf)
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

func createDomainData(domainDataService service.IDomainDataService,
	datasourceId string, t *testing.T) *datamesh.CreateDomainDataResponse {
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "int32"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: datasourceId,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	assert.NotNil(t, res)
	return res
}

func createDatasource(datasourceService service.IDomainDataSourceService,
	t *testing.T) *datamesh.CreateDomainDataSourceResponse {
	res := datasourceService.CreateDomainDataSource(context.Background(), &datamesh.CreateDomainDataSourceRequest{
		Header:       nil,
		DatasourceId: "",
		Name:         "default-data-source",
		Type:         "localfs",
		Info: &datamesh.DataSourceInfo{
			Localfs: &datamesh.LocalDataSourceInfo{
				Path: "./data",
			},
			Oss: nil,
		},
	})
	assert.NotNil(t, res)
	return res
}

func TestGetSchema(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	dsID = createTestDomainDataSource(t, conf)
	domainDataService := service.NewDomainDataService(conf)
	res := createDomainData(domainDataService, dsID, t)

	metaSrv := &DomainDataMetaServer{
		domainDataService:       domainDataService,
		domainDataSourceService: nil,
		dataServer:              nil,
	}

	query := &datamesh.CommandGetDomainDataSchema{
		DomaindataId: res.Data.DomaindataId,
	}

	schema, err := metaSrv.GetSchema(context.Background(), query)
	assert.Nil(t, err)

	arrowSchema, err := flight.DeserializeSchema(schema.GetSchema(), memory.DefaultAllocator)
	assert.Nil(t, err)
	assert.Equal(t, arrowSchema.Field(0).Type, arrow.PrimitiveTypes.Int32)
	assert.Equal(t, arrowSchema.Field(1).Type, arrow.BinaryTypes.String)
}

func TestCommandDomainDataQuery(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	dsID = createTestDomainDataSource(t, conf)
	domainDataService := service.NewDomainDataService(conf)

	datasourceService := service.NewDomainDataSourceService(conf)
	domainDataResp := createDomainData(domainDataService, dsID, t)

	metaSrv := &DomainDataMetaServer{
		domainDataService:       domainDataService,
		domainDataSourceService: datasourceService,
		dataServer:              &MockDataServer{},
	}

	query := &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataResp.Data.DomaindataId,
	}

	info, err := metaSrv.GetFlightInfoDomainDataQuery(context.Background(), query)
	assert.Nil(t, err)
	ticket := string(info.Endpoint[0].Ticket.Ticket)

	expected := fmt.Sprintf("%s-handler", domainDataResp.Data.DomaindataId)
	assert.Equal(t, ticket, expected)
}

func TestCommandDomainDataUpdate(t *testing.T) {
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
	}
	dsID = createTestDomainDataSource(t, conf)

	domainDataService := service.NewDomainDataService(conf)

	datasourceService := service.NewDomainDataSourceService(conf)

	metaSrv := &DomainDataMetaServer{
		domainDataService:       domainDataService,
		domainDataSourceService: datasourceService,
		dataServer:              &MockDataServer{},
	}

	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "int32"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	domainDataReq := &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	}

	update := &datamesh.CommandDomainDataUpdate{
		DomaindataRequest: domainDataReq,
	}

	info, err := metaSrv.GetFlightInfoDomainDataUpdate(context.Background(), update)
	assert.Nil(t, err)
	ticket := string(info.Endpoint[0].Ticket.Ticket)
	assert.True(t, len(ticket) > 0)
}
