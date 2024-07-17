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

package utils

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type DataMeshRequestContext struct {
	DataSourceType string
	Query          *datamesh.CommandDomainDataQuery
	Update         *datamesh.CommandDomainDataUpdate

	domainDataService       service.IDomainDataService
	domainDataSourceService service.IDomainDataSourceService
}

func NewDataMeshRequestContext(dd service.IDomainDataService, ds service.IDomainDataSourceService, msg proto.Message, dsType ...string) (*DataMeshRequestContext, error) {
	info := &DataMeshRequestContext{
		domainDataService:       dd,
		domainDataSourceService: ds,
	}

	switch msg := msg.(type) {
	case *datamesh.CommandDomainDataQuery:
		info.Query = msg
	case *datamesh.CommandDomainDataUpdate:
		info.Update = msg
	default:
		return nil, status.Error(codes.InvalidArgument, "FlightDescriptor.Cmd of GetFlightInfo Request is invalid, need CommandDomainDataQuery/CommandDomainDataUpdate")
	}
	// init datasource type
	if len(dsType) != 0 {
		info.DataSourceType = dsType[0]
	} else {
		if ds != nil && dd != nil {
			dds, err := info.GetDomainDataSource(context.Background())
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			info.DataSourceType = dds.GetType()
		}
	}
	return info, nil
}

func (rc *DataMeshRequestContext) GetDomainData(ctx context.Context) (*datamesh.DomainData, error) {
	domainDataReq := &datamesh.QueryDomainDataRequest{
		DomaindataId: rc.getDomainDataID(),
	}

	domainDataResp := rc.domainDataService.QueryDomainData(ctx, domainDataReq)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "Query domain data by id(%s) fail", rc.getDomainDataID())
	}
	return domainDataResp.Data, nil
}

func (rc *DataMeshRequestContext) GetDomainDataAndSource(ctx context.Context) (*datamesh.DomainData, *datamesh.DomainDataSource, error) {
	var data *datamesh.DomainData
	var err error
	if data, err = rc.GetDomainData(ctx); err != nil {
		return nil, nil, err
	}

	datasourceReq := &datamesh.QueryDomainDataSourceRequest{
		DatasourceId: data.DatasourceId,
	}

	datasourceResp := rc.domainDataSourceService.QueryDomainDataSource(ctx, datasourceReq)
	if datasourceResp == nil || datasourceResp.GetStatus() == nil || datasourceResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if datasourceResp.GetStatus() != nil {
			appStatus = datasourceResp.GetStatus()
		}
		return data, nil, common.BuildGrpcErrorf(appStatus, codes.FailedPrecondition, "Query data source by id(%s) fail", datasourceReq.DatasourceId)
	}
	return data, datasourceResp.GetData(), nil
}

func (rc *DataMeshRequestContext) GetDomainDataSource(ctx context.Context) (*datamesh.DomainDataSource, error) {
	_, ds, err := rc.GetDomainDataAndSource(ctx)
	return ds, err
}

func (rc *DataMeshRequestContext) getDomainDataID() string {
	if rc.Query != nil {
		return rc.Query.DomaindataId
	}

	return rc.Update.DomaindataId
}

func (rc *DataMeshRequestContext) GetTransferContentType() datamesh.ContentType {
	if rc.Query != nil {
		return rc.Query.ContentType
	} else if rc.Update != nil {
		return rc.Update.ContentType
	}

	return datamesh.ContentType_RAW
}
