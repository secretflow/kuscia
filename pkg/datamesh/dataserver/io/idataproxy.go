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

package io

import (
	"context"

	"github.com/apache/arrow/go/v13/arrow/flight"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/io/builtin"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/io/external"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
)

type Server interface {
	GetFlightInfo(ctx context.Context, reqCtx *utils.DataMeshRequestContext) (flightInfo *flight.FlightInfo, err error)
	DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) (err error)
	DoPut(stream flight.FlightService_DoPutServer) (err error)
}

func NewExternalIO(conf *config.DataProxyConfig) Server {
	return external.NewIOServer(conf)
}

func NewBuiltinIO() Server {
	return builtin.NewIOServer()
}
