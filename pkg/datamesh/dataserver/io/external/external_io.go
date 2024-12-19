// Copyright 2024 Ant Group Co., Ltd.
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

package external

import (
	"context"
	"errors"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

var partitionSpecKey = "partition_spec"

type IOServer struct {
	exDpClient IDataProxyClient
}

func NewIOServer(conf *config.DataProxyConfig) *IOServer {
	cli, err := NewDataProxyClient(conf)
	if err != nil {
		nlog.Fatalf("New external data proxy endpoints:%s failed, error: %s.", conf.Endpoint, err.Error())
	}
	return &IOServer{exDpClient: cli}
}

func (d *IOServer) GetFlightInfo(ctx context.Context, reqCtx *utils.DataMeshRequestContext) (flightInfo *flight.FlightInfo, err error) {
	dd, ds, err := reqCtx.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("GetFlightInfo get DomainData and Source failed, error: %s.", err.Error())
		return nil, err
	}
	if reqCtx.Query != nil {
		req := &datamesh.CommandDataMeshQuery{
			Query:      reqCtx.Query,
			Domaindata: dd,
			Datasource: ds,
		}
		if partitionSpec, ok := dd.Attributes[partitionSpecKey]; ok {
			req.Query.PartitionSpec = partitionSpec
		}
		return d.exDpClient.GetFlightInfoDataMeshQuery(ctx, req)
	}
	if reqCtx.Update != nil {
		req := &datamesh.CommandDataMeshUpdate{
			Update:     reqCtx.Update,
			Domaindata: dd,
			Datasource: ds,
		}
		if partitionSpec, ok := dd.Attributes[partitionSpecKey]; ok {
			req.Update.PartitionSpec = partitionSpec
		}
		return d.exDpClient.GetFlightInfoDataMeshUpdate(ctx, req)
	}

	if reqCtx.SqlQuery != nil {
		req := &datamesh.CommandDataMeshSqlQuery{
			Query:      reqCtx.SqlQuery,
			Datasource: ds,
		}
		return d.exDpClient.GetFlightInfoDataMeshSqlQuery(ctx, req)
	}

	return nil, status.Errorf(codes.InvalidArgument, "Request is not query or update.")
}

func (d *IOServer) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (err error) {
	// no need implement
	return errors.New("external DoGet not implement")
}

func (d *IOServer) DoPut(stream flight.FlightService_DoPutServer) (err error) {
	// no need implement
	return errors.New("external DoPut not implement")
}
