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
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/memory"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type MetaServer interface {
	GetSchema(context.Context, *datamesh.CommandGetDomainDataSchema) (*flight.SchemaResult, error)
	GetFlightInfoDomainDataQuery(context.Context, *datamesh.CommandDomainDataQuery) (*flight.FlightInfo, error)
	GetFlightInfoDomainDataUpdate(context.Context, *datamesh.CommandDomainDataUpdate) (*flight.FlightInfo, error)
	DoActionCreateDomainDataRequest(ctx context.Context, request *datamesh.ActionCreateDomainDataRequest) (*flight.Result, error)
	DoActionQueryDomainDataRequest(ctx context.Context, request *datamesh.ActionQueryDomainDataRequest) (*flight.Result, error)
	DoActionUpdateDomainDataRequest(ctx context.Context, request *datamesh.ActionUpdateDomainDataRequest) (*flight.Result, error)
	DoActionDeleteDomainDataRequest(ctx context.Context, request *datamesh.ActionDeleteDomainDataRequest) (*flight.Result, error)
	DoActionCreateDomainDataSourceRequest(ctx context.Context, request *datamesh.ActionCreateDomainDataSourceRequest) (*flight.Result, error)
	DoActionQueryDomainDataSourceRequest(ctx context.Context, request *datamesh.ActionQueryDomainDataSourceRequest) (
		*flight.Result, error)
}

type DomainDataMetaServer struct {
	domainDataService       service.IDomainDataService
	domainDataSourceService service.IDomainDataSourceService
	dataServer              DataServer
	datamesh.UnimplementedDomainDataServiceServer
	datamesh.UnimplementedDomainDataSourceServiceServer
}

func NewMetaServer(domainDataService service.IDomainDataService, dataSourceService service.IDomainDataSourceService,
	config *DataProxyConfig) (*DomainDataMetaServer, error) {
	dataServer, err := NewDataProxyClient(config)
	if err != nil {
		return nil, err
	}
	return &DomainDataMetaServer{
		domainDataService:       domainDataService,
		domainDataSourceService: dataSourceService,
		dataServer:              dataServer,
	}, nil
}

func (s *DomainDataMetaServer) GetSchema(ctx context.Context, query *datamesh.CommandGetDomainDataSchema) (*flight.SchemaResult, error) {
	domainDataResp, err := s.getDomainData(ctx, query.DomaindataId)
	if err != nil {
		return nil, err
	}
	domainData := domainDataResp.Data

	var fields []arrow.Field
	for _, column := range domainData.Columns {
		colType := Convert2ArrowColumnType(column.Type)
		if colType == nil {
			return nil, buildGrpcErrorf(nil, codes.FailedPrecondition, "invalid column(%s）with type(%s)", column.Name,
				column.Type)
		}
		fields = append(fields, arrow.Field{
			Name:     column.Name,
			Type:     colType,
			Nullable: !column.NotNullable,
		})
	}

	metadata := arrow.NewMetadata([]string{"type"}, []string{domainData.Type})
	result := &flight.SchemaResult{
		Schema: flight.SerializeSchema(arrow.NewSchema(fields, &metadata), memory.DefaultAllocator),
	}
	return result, nil
}

func (s *DomainDataMetaServer) GetFlightInfoDomainDataQuery(ctx context.Context,
	query *datamesh.CommandDomainDataQuery) (*flight.FlightInfo, error) {
	domainDataResp, err := s.getDomainData(ctx, query.DomaindataId)
	if err != nil {
		return nil, err
	}

	datasource, err := s.getDataSource(ctx, domainDataResp.Data.GetDatasourceId())
	if err != nil {
		return nil, err
	}

	// adjust ContentType
	if strings.ToLower(domainDataResp.Data.Type) != "table" {
		query.ContentType = datamesh.ContentType_RAW
	}

	dpQuery := &datamesh.CommandDataMeshQuery{
		Query:      query,
		Domaindata: domainDataResp.Data,
		Datasource: datasource,
	}

	return s.dataServer.GetFlightInfoDataMeshQuery(ctx, dpQuery)
}

func (s *DomainDataMetaServer) GetFlightInfoDomainDataUpdate(ctx context.Context,
	query *datamesh.CommandDomainDataUpdate) (*flight.FlightInfo, error) {
	var (
		domainData *datamesh.DomainData
		datasource *datamesh.DomainDataSource
		err        error

		domainDataResp *datamesh.QueryDomainDataResponse
	)

	domainDataID := query.DomaindataId

	if query.GetDomaindataRequest() != nil {
		if _, err = s.createDomainData(ctx, query.GetDomaindataRequest()); err != nil {
			return nil, err
		}
		domainDataID = query.GetDomaindataRequest().DomaindataId
	}

	if domainDataResp, err = s.getDomainData(ctx, domainDataID); err != nil {
		return nil, err
	}
	domainData = domainDataResp.Data

	if datasource, err = s.getDataSource(ctx, domainData.GetDatasourceId()); err != nil {
		return nil, err
	}

	// adjust ContentType
	if strings.ToLower(domainDataResp.Data.Type) != "table" {
		query.ContentType = datamesh.ContentType_RAW
	}

	dpUpdate := &datamesh.CommandDataMeshUpdate{
		Update:     query,
		Domaindata: domainData,
		Datasource: datasource,
	}

	return s.dataServer.GetFlightInfoDataMeshUpdate(ctx, dpUpdate)
}

func (s *DomainDataMetaServer) DoActionCreateDomainDataRequest(ctx context.Context, request *datamesh.ActionCreateDomainDataRequest) (*flight.
	Result, error) {
	domainDataResp, err := s.createDomainData(ctx, request.Request)
	if err != nil {
		return nil, err
	}

	actionResp := &datamesh.ActionCreateDomainDataResponse{
		Response: domainDataResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) DoActionQueryDomainDataRequest(ctx context.Context,
	request *datamesh.ActionQueryDomainDataRequest) (*flight.Result, error) {
	domainDataResp, err := s.getDomainData(ctx, request.Request.DomaindataId)
	if err != nil {
		return nil, err
	}

	actionResp := &datamesh.ActionQueryDomainDataResponse{
		Response: domainDataResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) DoActionUpdateDomainDataRequest(ctx context.Context,
	request *datamesh.ActionUpdateDomainDataRequest) (*flight.Result, error) {
	domainDataResp := s.domainDataService.UpdateDomainData(ctx, request.Request)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "update domain data with id(%s) fail", request.Request.DomaindataId)
	}

	actionResp := &datamesh.ActionUpdateDomainDataResponse{
		Response: domainDataResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) DoActionDeleteDomainDataRequest(ctx context.Context,
	request *datamesh.ActionDeleteDomainDataRequest) (*flight.Result, error) {
	domainDataResp := s.domainDataService.DeleteDomainData(ctx, request.Request)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "delete domain data with id(%s) fail",
			request.Request.DomaindataId)
	}

	actionResp := &datamesh.ActionDeleteDomainDataResponse{
		Response: domainDataResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) DoActionCreateDomainDataSourceRequest(ctx context.Context,
	request *datamesh.ActionCreateDomainDataSourceRequest) (*flight.Result, error) {
	domainSourceResp := s.domainDataSourceService.CreateDomainDataSource(ctx, request.Request)
	if domainSourceResp == nil || domainSourceResp.GetStatus() == nil || domainSourceResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainSourceResp.GetStatus() != nil {
			appStatus = domainSourceResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "update datasource data with id(%s) fail",
			request.Request)
	}

	actionResp := &datamesh.ActionCreateDomainDataSourceResponse{
		Response: domainSourceResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) DoActionQueryDomainDataSourceRequest(ctx context.Context,
	request *datamesh.ActionQueryDomainDataSourceRequest) (*flight.Result, error) {
	domainSourceResp := s.domainDataSourceService.QueryDomainDataSource(ctx, request.Request)
	if domainSourceResp == nil || domainSourceResp.GetStatus() == nil || domainSourceResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainSourceResp.GetStatus() != nil {
			appStatus = domainSourceResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "query datasource with id(%s) fail",
			request.Request.DatasourceId)
	}

	actionResp := &datamesh.ActionQueryDomainDataSourceResponse{
		Response: domainSourceResp,
	}
	return packActionResult(actionResp)
}

func (s *DomainDataMetaServer) getDomainData(ctx context.Context, id string) (*datamesh.QueryDomainDataResponse, error) {
	domainDataReq := &datamesh.QueryDomainDataRequest{
		DomaindataId: id,
	}

	domainDataResp := s.domainDataService.QueryDomainData(ctx, domainDataReq)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "query domain data by id(%s) fail", id)
	}
	return domainDataResp, nil
}

func (s *DomainDataMetaServer) createDomainData(ctx context.Context,
	domainDataReq *datamesh.CreateDomainDataRequest) (*datamesh.CreateDomainDataResponse, error) {
	domainDataResp := s.domainDataService.CreateDomainData(ctx, domainDataReq)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "create domain data with id(%s) fail",
			domainDataReq.DomaindataId)
	}
	return domainDataResp, nil
}

func (s *DomainDataMetaServer) getDataSource(ctx context.Context, id string) (*datamesh.DomainDataSource, error) {
	datasourceReq := &datamesh.QueryDomainDataSourceRequest{
		DatasourceId: id,
	}

	datasourceResp := s.domainDataSourceService.QueryDomainDataSource(ctx, datasourceReq)
	if datasourceResp == nil || datasourceResp.GetStatus() == nil || datasourceResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if datasourceResp.GetStatus() != nil {
			appStatus = datasourceResp.GetStatus()
		}
		return nil, buildGrpcErrorf(appStatus, codes.Internal, "query data source by id(%s) fail", datasourceReq.DatasourceId)
	}
	return datasourceResp.GetData(), nil
}

func buildGrpcErrorf(appStatus *v1alpha1.Status, code codes.Code, format string, a ...interface{}) error {
	if appStatus == nil {
		nlog.Warnf(format, a...)
		return status.Errorf(code, format, a...)
	}

	s := &spb.Status{
		Code: int32(code),
	}
	if len(appStatus.Message) > 0 {
		s.Message = appStatus.Message
	} else {
		s.Message = fmt.Sprintf(format, a...)
	}
	nlog.Warnf("%s", s.Message)

	if detail, err := anypb.New(appStatus); err != nil {
		s.Details = []*anypb.Any{
			detail,
		}
	}
	return status.ErrorProto(s)
}

func packActionResult(msg proto.Message) (*flight.Result, error) {
	var (
		anyCmd anypb.Any
		err    error
	)

	if err = anyCmd.MarshalFrom(msg); err != nil {
		nlog.Warnf("unable to marshal final response: %v", err)
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("%v: unable to marshal final response", err))
	}

	ret := &flight.Result{}
	if ret.Body, err = proto.Marshal(&anyCmd); err != nil {
		nlog.Warnf("unable to marshal final response: %v", err)
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("%v: unable to marshal final response", err))
	}
	return ret, nil
}
