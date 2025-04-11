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

package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type IOServer struct {
	ioChannels map[string]DataMeshDataIOInterface
	cmds       *gocache.Cache
}

func NewIOServer() *IOServer {
	return &IOServer{
		cmds: gocache.New(time.Duration(10)*time.Minute, time.Minute),
		ioChannels: map[string]DataMeshDataIOInterface{
			common.DomainDataSourceTypeLocalFS:    NewBuiltinLocalFileIOChannel(),
			common.DomainDataSourceTypeOSS:        NewBuiltinOssIOChannel(),
			common.DomainDataSourceTypeMysql:      NewBuiltinMySQLIOChannel(),
			common.DomainDataSourceTypePostgreSQL: NewBuiltinPostgresIOChannel(),
		},
	}
}

func (d *IOServer) GetFlightInfo(ctx context.Context, reqCtx *utils.DataMeshRequestContext) (flightInfo *flight.FlightInfo, err error) {
	tickUUID := uuid.New().String()
	dataSource, err := reqCtx.GetDomainDataSource(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get datasource type failed with %s", err.Error())
	}
	channel, ok := d.ioChannels[dataSource.Type]
	if !ok {
		nlog.Warnf("Not found io channel for datasource type: %s", dataSource.Type)
		return nil, status.Errorf(codes.InvalidArgument, "datasource type (%s) not supported", dataSource.Type)
	}

	if err := d.cmds.Add(tickUUID, reqCtx, 10*time.Minute); err != nil {
		return nil, status.Error(codes.Internal,
			fmt.Sprintf("cache domaindata failed, please try call GetFlightInfo again. raw message=(%s)", err.Error()))
	}

	nlog.Infof("[DataMesh] [GetFlightInfo] ticket=%s", tickUUID)

	return utils.CreateDateMeshFlightInfo([]byte(tickUUID), channel.GetEndpointURI()), nil
}

func (d *IOServer) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoGet] recover with error %v", r)
			err = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
		}
	}()

	ticketID := string(tkt.Ticket)
	nlog.Infof("[DataMesh] [DoGet] ticket=%s", ticketID)
	reqContext, ok := d.cmds.Get(ticketID)
	if !ok || reqContext == nil {
		nlog.Warnf("[DataMesh] [DoGet] invalidate input ticket=%s", ticketID)
		return status.Errorf(codes.InvalidArgument, "invalid ticket:%s", ticketID)
	}

	reqCtx := reqContext.(*utils.DataMeshRequestContext)

	var w utils.RecordWriter
	if reqCtx.GetTransferContentType() == datamesh.ContentType_RAW {
		w = flight.NewRecordWriter(fs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
	} else {
		data, err := reqCtx.GetDomainData(context.Background())
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		schema, err := utils.GenerateArrowSchema(data)
		if err != nil {
			nlog.Errorf("Domaindata(%s) generate arrow schema error: %s", data.GetDomaindataId(), err.Error())
			return status.Errorf(codes.Internal, "generate arrow schema failed with %s", err.Error())
		}
		flightWriter := flight.NewRecordWriter(fs, ipc.WithSchema(schema))
		w = &utils.FlightRecordWriter{
			FlightWriter: fs,
			Writer:       flightWriter,
		}
	}
	if ios, ok := d.ioChannels[reqCtx.DataSourceType]; ok {
		if ioReadErr := ios.Read(fs.Context(), reqCtx, w); ioReadErr != nil {
			nlog.Errorf("Read domaindata failed with %s", ioReadErr.Error())
			return status.Error(codes.Internal, fmt.Sprintf("Read domaindata failed with %s", ioReadErr.Error()))
		}
		return nil
	}
	nlog.Errorf("The datasource type (%s) not found in io channels ", reqCtx.DataSourceType)
	return status.Errorf(codes.Internal, "The datasource type (%s) not found in io channels ", reqCtx.DataSourceType)
}

func (d *IOServer) DoPut(stream flight.FlightService_DoPutServer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Warnf("[DataMesh] [DoPut] recover with error %v", r)
			err = status.Error(codes.Internal, fmt.Sprintf("unknown error, msg=%v", r))
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
	reqContext, ok := d.cmds.Get(ticketID)
	if !ok || reqContext == nil {
		nlog.Warnf("[DataMesh] [DoPut] invalidate input ticket=%s", ticketID)
		return status.Errorf(codes.InvalidArgument, fmt.Sprintf("invalid ticket:%s", ticketID))
	}
	reqCtx := reqContext.(*utils.DataMeshRequestContext)
	if ios, ok := d.ioChannels[reqCtx.DataSourceType]; ok {

		if ioWriteErr := ios.Write(stream.Context(), reqCtx, reader); ioWriteErr != nil {
			nlog.Errorf("Write domaindata failed with %s", ioWriteErr.Error())
			return status.Error(codes.Internal, fmt.Sprintf("Write domaindata failed with %s", ioWriteErr.Error()))
		}
		return nil
	}
	nlog.Errorf("The datasource type (%s) not found in io channels ", reqCtx.DataSourceType)
	return status.Errorf(codes.Internal, "The datasource type (%s) not found in io channels ", reqCtx.DataSourceType)
}
