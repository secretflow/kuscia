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

package dmflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/google/uuid"
	cache "github.com/patrickmn/go-cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type CustomActionHandler func(context.Context, []byte) (*flight.Result, error)

type datameshFlightHandler struct {
	flight.BaseFlightServer
	cmds *cache.Cache

	iochannels map[string]DataMeshDataIOInterface

	customHandles map[string]CustomActionHandler

	domainDataService       service.IDomainDataService
	domainDataSourceService service.IDomainDataSourceService
}

func NewDataMeshFlightHandler(dds service.IDomainDataService, dss service.IDomainDataSourceService, configs []config.ExternalDataProxyConfig) flight.FlightServer {
	handler := &datameshFlightHandler{
		cmds:          cache.New(time.Minute*10, time.Minute),
		customHandles: map[string]CustomActionHandler{},
		iochannels: map[string]DataMeshDataIOInterface{
			"localfs": NewBuiltinLocalFileIOChannel(),
			"oss":     NewBuiltinOssIOChannel(),
		},
		domainDataService:       dds,
		domainDataSourceService: dss,
	}

	chs := NewDataMeshCustomActionHandlers(dds, dss)
	handler.customHandles["ActionCreateDomainDataRequest"] = chs.DoActionCreateDomainDataRequest
	handler.customHandles["ActionQueryDomainDataRequest"] = chs.DoActionQueryDomainDataRequest
	handler.customHandles["ActionUpdateDomainDataRequest"] = chs.DoActionUpdateDomainDataRequest
	handler.customHandles["ActionDeleteDomainDataRequest"] = chs.DoActionDeleteDomainDataRequest
	handler.customHandles["ActionQueryDomainDataSourceRequest"] = chs.DoActionQueryDomainDataSourceRequest

	for _, cfg := range configs {
		for _, dstype := range cfg.DataSourceTypes {
			nlog.Infof("Extenal dataproxy type[%s] mode(%s) %s", dstype, cfg.Mode, cfg.Endpoint)

			if strings.ToLower(cfg.Mode) == string(config.ModeDirect) {
				handler.iochannels[dstype] = NewBuiltinDirectIO(&cfg)
			}
		}
	}
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
	var (
		anyCmd anypb.Any
		info   *DataMeshRequestContext
	)

	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [GetFlightInfo] recover with error %v", r)
			ret = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()

	if err := proto.Unmarshal(request.Cmd, &anyCmd); err != nil {
		nlog.Warnf("Unable to parse FlightDescriptor.Cmd to Any")
		return nil, status.Errorf(codes.InvalidArgument, "unable to parse command: %v", err)
	}

	msg, err := anyCmd.UnmarshalNew()
	if err != nil {
		nlog.Warnf("FlightDescriptor.Cmd UnmarshalNew fail: %s", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal Any to a command type: %s", err.Error())
	}

	if info, err = NewDataMeshRequestContext(f.domainDataService, f.domainDataSourceService, msg); err != nil {
		nlog.Warnf("GetFlightInfo create context fail: %s", err.Error())
		return nil, err
	}

	tickUUID := uuid.New().String()

	dataSource, err := info.GetDomainDataSource(context.Background())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get datasource type failed with %s", err.Error())
	}
	channel, ok := f.iochannels[dataSource.Type]
	if !ok {
		nlog.Warnf("Not found io channel for datasource type: %s", dataSource.Type)
		return nil, status.Errorf(codes.InvalidArgument, "datasource type (%s) not supported", dataSource.Type)
	}

	info.io = channel

	if err := f.cmds.Add(string(tickUUID), info, 10*time.Minute); err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("cache domaindata failed, please try call GetFlightInfo again. raw message=(%s)", err.Error()))
	}

	nlog.Infof("[DataMesh] [GetFlightInfo] ticket=%s", string(tickUUID))

	return CreateDateMeshFlightInfo([]byte(tickUUID), channel.GetEndpointURI()), nil
}

func (f *datameshFlightHandler) DoAction(action *flight.Action, stream flight.FlightService_DoActionServer) (ret error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoAction] recover with error %v", r)
			ret = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
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

func (f *datameshFlightHandler) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (ret error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoGet] recover with error %v", r)
			ret = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()

	ticketID := string(tkt.Ticket)
	nlog.Infof("[DataMesh] [DoGet] ticket=%s", ticketID)
	channel, ok := f.cmds.Get(ticketID)
	if !ok || channel == nil {
		nlog.Warnf("[DataMesh] [DoGet] invalidate input ticket=%s", ticketID)
		return status.Errorf(codes.InvalidArgument, "invalid ticket:%s", ticketID)
	}

	ioInfo := channel.(*DataMeshRequestContext)

	var w *flight.Writer
	if ioInfo.GetTransferContentType() == datamesh.ContentType_RAW {
		w = flight.NewRecordWriter(fs, ipc.WithSchema(GenerateBinaryDataArrowSchema()))
	} else {
		data, err := ioInfo.GetDomainData(context.Background())
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		schema, err := GenerateArrowSchema(data)
		if err != nil {
			nlog.Errorf("Domaindata(%s) generate arrow schema error: %s", data.GetDomaindataId(), err.Error())
			return status.Errorf(codes.Internal, "generate arrow schema failed with %s", err.Error())
		}
		w = flight.NewRecordWriter(fs, ipc.WithSchema(schema))
	}

	defer w.Close()

	return ioInfo.io.Read(fs.Context(), ioInfo, w)

}

func (f *datameshFlightHandler) DoPut(stream flight.FlightService_DoPutServer) (ret error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoPut] recover with error %v", r)
			ret = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()

	reader, err := flight.NewRecordReader(stream)
	if err != nil {
		nlog.Warnf("[DataMesh] [DoPut] create record reader failed with %s", err.Error())
		return status.Error(codes.Internal, err.Error())
	}
	defer reader.Release()

	desc := reader.LatestFlightDescriptor()
	if desc == nil {
		nlog.Warnf("[DataMesh] [DoPut] not found Descriptor")
		return status.Error(codes.InvalidArgument, "not found Descriptor")
	}

	ticketID := string(desc.Cmd)

	nlog.Infof("[DataMesh] [DoPut] ticket=%s", ticketID)

	channel, ok := f.cmds.Get(ticketID)
	if !ok || channel == nil {
		nlog.Warnf("[DataMesh] [DoPut] invalidate input ticket=%s", ticketID)
		return status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid ticket:%s", ticketID))
	}

	ioInfo := channel.(*DataMeshRequestContext)

	return ioInfo.io.Write(stream.Context(), ioInfo, reader)
}
