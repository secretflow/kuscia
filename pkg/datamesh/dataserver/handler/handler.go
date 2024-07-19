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

package handler

import (
	"context"
	"fmt"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	svc "github.com/secretflow/kuscia/pkg/datamesh/dataserver/service"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type CustomActionHandler func(context.Context, []byte) (*flight.Result, error)

type datameshFlightHandler struct {
	flight.BaseFlightServer
	customHandles           map[string]CustomActionHandler
	domainDataService       service.IDomainDataService
	domainDataSourceService service.IDomainDataSourceService
	flightService           *svc.FlightIO
}

func NewDataMeshFlightHandler(dds service.IDomainDataService, dss service.IDomainDataSourceService, configs []config.DataProxyConfig) flight.FlightServer {
	handler := &datameshFlightHandler{
		customHandles:           map[string]CustomActionHandler{},
		domainDataService:       dds,
		domainDataSourceService: dss,
	}
	// new dp flight
	handler.flightService = svc.NewFlightIO(dds, dss, configs)
	chs := svc.NewCustomActionService(dds, dss)
	handler.customHandles["ActionCreateDomainDataRequest"] = chs.DoActionCreateDomainDataRequest
	handler.customHandles["ActionQueryDomainDataRequest"] = chs.DoActionQueryDomainDataRequest
	handler.customHandles["ActionUpdateDomainDataRequest"] = chs.DoActionUpdateDomainDataRequest
	handler.customHandles["ActionDeleteDomainDataRequest"] = chs.DoActionDeleteDomainDataRequest
	handler.customHandles["ActionQueryDomainDataSourceRequest"] = chs.DoActionQueryDomainDataSourceRequest
	return handler
}

func (f *datameshFlightHandler) GetSchema(ctx context.Context, request *flight.FlightDescriptor) (*flight.SchemaResult, error) {
	var (
		anyCmd anypb.Any
		msg    proto.Message
		err    error
	)

	if err = proto.Unmarshal(request.Cmd, &anyCmd); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unable to parse command: %s", err.Error())
	}

	if msg, err = anyCmd.UnmarshalNew(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal Any to a command type: %s", err.Error())
	}

	switch cmd := msg.(type) {
	case *datamesh.CommandGetDomainDataSchema:
		res := f.domainDataService.QueryDomainData(ctx, &datamesh.QueryDomainDataRequest{
			DomaindataId: cmd.DomaindataId,
		})
		if res.GetStatus() != nil && res.GetStatus().GetCode() != 0 {
			return nil, common.BuildGrpcErrorf(res.GetStatus(), codes.Internal, "query domain data by id(%s) fail", cmd.DomaindataId)
		}

		domainData := res.Data

		var fields []arrow.Field
		for _, column := range domainData.Columns {
			colType := common.Convert2ArrowColumnType(column.Type)
			if colType == nil {
				return nil, common.BuildGrpcErrorf(nil, codes.FailedPrecondition, "invalid column(%s) with type(%s)", column.Name,
					column.Type)
			}
			fields = append(fields, arrow.Field{
				Name:     column.Name,
				Type:     colType,
				Nullable: !column.NotNullable,
			})
		}

		metadata := arrow.NewMetadata([]string{"type"}, []string{domainData.Type})
		return &flight.SchemaResult{
			Schema: flight.SerializeSchema(arrow.NewSchema(fields, &metadata), memory.DefaultAllocator),
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "request command is invalid")
}

func (f *datameshFlightHandler) GetFlightInfo(ctx context.Context, request *flight.FlightDescriptor) (fi *flight.FlightInfo, ret error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [GetFlightInfo] recover with error %v", r)
			ret = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()
	var anyCmd anypb.Any
	if err := proto.Unmarshal(request.Cmd, &anyCmd); err != nil {
		nlog.Warnf("Unable to parse FlightDescriptor.Cmd to Any")
		return nil, status.Errorf(codes.InvalidArgument, "unable to parse command: %v", err)
	}
	msg, err := anyCmd.UnmarshalNew()
	if err != nil {
		nlog.Warnf("FlightDescriptor.Cmd UnmarshalNew fail: %s", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal Any to a command type: %s", err.Error())
	}
	return f.flightService.GetFlightInfo(ctx, msg)
}

func (f *datameshFlightHandler) DoAction(action *flight.Action, stream flight.FlightService_DoActionServer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoAction] recover with error %v", r)
			err = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()

	if cah, ok := f.customHandles[action.Type]; ok {
		result, err := cah(context.Background(), action.GetBody())
		if err != nil {
			nlog.Warnf("[DataMesh] process action(%s) failed with error: %s", action.GetType(), err.Error())
			return err
		}

		return stream.Send(result)
	}

	return status.Errorf(codes.InvalidArgument, "unsupported action type: %s", action.Type)
}

func (f *datameshFlightHandler) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (err error) {
	return f.flightService.DoGet(tkt, fs)
}

func (f *datameshFlightHandler) DoPut(stream flight.FlightService_DoPutServer) (err error) {
	return f.flightService.DoPut(stream)
}
