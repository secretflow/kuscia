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

package service

import (
	"context"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/io"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type FlightIO struct {
	dd    service.IDomainDataService
	ds    service.IDomainDataSourceService
	ioMap map[string]io.Server
	inIO  io.Server
}

func NewFlightIO(dd service.IDomainDataService, ds service.IDomainDataSourceService, configs []config.DataProxyConfig) *FlightIO {
	inIO := io.NewBuiltinIO()
	fs := FlightIO{
		dd: dd,
		ds: ds,
		ioMap: map[string]io.Server{
			common.DomainDataSourceTypeLocalFS: inIO,
			common.DomainDataSourceTypeOSS:     inIO,
			common.DomainDataSourceTypeMysql:   inIO,
		},
		inIO: inIO,
	}
	for _, conf := range configs {
		exDp := io.NewExternalIO(&conf)
		for _, typ := range conf.DataSourceTypes {
			nlog.Infof("External dataproxy type[%s] mode(%s) %s", typ, conf.Mode, conf.Endpoint)
			fs.ioMap[typ] = exDp
		}
	}
	return &fs
}

func (dp *FlightIO) GetFlightInfo(ctx context.Context, msg proto.Message) (flightInfo *flight.FlightInfo, err error) {
	reqCtx, err := utils.NewDataMeshRequestContext(dp.dd, dp.ds, msg)
	if err != nil {
		nlog.Warnf("GetFlightInfo create context fail: %s", err.Error())
		return nil, err
	}

	if dpX, ok := dp.ioMap[reqCtx.DataSourceType]; ok {
		return dpX.GetFlightInfo(ctx, reqCtx)
	}
	return nil, status.Errorf(codes.InvalidArgument, "datasource type (%s) without data proxy", reqCtx.DataSourceType)
}

func (dp *FlightIO) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (err error) {
	return dp.inIO.DoGet(tkt, fs)
}

func (dp *FlightIO) DoPut(stream flight.FlightService_DoPutServer) (err error) {
	return dp.inIO.DoPut(stream)
}
