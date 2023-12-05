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

package grpchandler

import (
	"context"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	flight2 "github.com/secretflow/kuscia/pkg/datamesh/flight"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type FlightMetaHandler struct {
	flight.BaseFlightServer
	srv flight2.MetaServer
}

func NewFlightMetaHandler(srv flight2.MetaServer) *FlightMetaHandler {
	return &FlightMetaHandler{
		srv: srv,
	}
}

func (f *FlightMetaHandler) GetSchema(ctx context.Context, request *flight.FlightDescriptor) (*flight.SchemaResult,
	error) {
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
		return f.srv.GetSchema(ctx, cmd)
	}

	return nil, status.Error(codes.InvalidArgument, "request command is invalid")
}

func (f *FlightMetaHandler) GetFlightInfo(ctx context.Context, request *flight.FlightDescriptor) (*flight.FlightInfo,
	error) {
	var (
		anyCmd     anypb.Any
		msg        proto.Message
		err        error
		flightInfo *flight.FlightInfo
	)

	if err = proto.Unmarshal(request.Cmd, &anyCmd); err != nil {
		nlog.Warnf("Unable to parse FlightDescriptor.Cmd to Any")
		return nil, status.Errorf(codes.InvalidArgument, "unable to parse command: %v", err)
	}

	if msg, err = anyCmd.UnmarshalNew(); err != nil {
		nlog.Warnf("FlightDescriptor.Cmd UnmarshalNew fail: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal Any to a command type: %s", err.Error())
	}

	switch cmd := msg.(type) {
	case *datamesh.CommandDomainDataQuery:
		flightInfo, err = f.srv.GetFlightInfoDomainDataQuery(ctx, cmd)
	case *datamesh.CommandDomainDataUpdate:
		flightInfo, err = f.srv.GetFlightInfoDomainDataUpdate(ctx, cmd)
	default:
		err = status.Error(codes.InvalidArgument, "FlightDescriptor.Cmd of GetFlightInfo Request is invalid")
	}

	if err != nil {
		nlog.Warnf("GetFlightInfo fail: %v", err)
	}
	return flightInfo, err
}

func (f *FlightMetaHandler) DoAction(action *flight.Action, stream flight.FlightService_DoActionServer) error {
	var (
		result *flight.Result
		err    error
	)

	buildUnmarshalError := func() error {
		nlog.Warnf("Action Request body can not deserialize to %s, body size:%d", action.Type, len(action.Body))
		return status.Errorf(codes.InvalidArgument, "request action body can not deserialize to %s", action.Type)
	}

	switch action.Type {
	case "ActionCreateDomainDataRequest":
		var (
			request datamesh.ActionCreateDomainDataRequest
		)
		if err = proto.Unmarshal(action.Body, &request); err != nil {
			return buildUnmarshalError()
		}
		if result, err = f.srv.DoActionCreateDomainDataRequest(context.Background(), &request); err != nil {
			return err
		}
	case "ActionQueryDomainDataRequest":
		var (
			request datamesh.ActionQueryDomainDataRequest
		)
		if err = proto.Unmarshal(action.Body, &request); err != nil {
			return buildUnmarshalError()
		}
		result, err = f.srv.DoActionQueryDomainDataRequest(context.Background(), &request)
	case "ActionUpdateDomainDataRequest":
		var (
			request datamesh.ActionUpdateDomainDataRequest
		)
		if err = proto.Unmarshal(action.Body, &request); err != nil {
			return buildUnmarshalError()
		}
		result, err = f.srv.DoActionUpdateDomainDataRequest(context.Background(), &request)
	case "ActionDeleteDomainDataRequest":
		var (
			request datamesh.ActionDeleteDomainDataRequest
		)
		if err = proto.Unmarshal(action.Body, &request); err != nil {
			return buildUnmarshalError()
		}
		result, err = f.srv.DoActionDeleteDomainDataRequest(context.Background(), &request)
	case "ActionQueryDomainDataSourceRequest":
		var (
			request datamesh.ActionQueryDomainDataSourceRequest
		)
		if err = proto.Unmarshal(action.Body, &request); err != nil {
			return buildUnmarshalError()
		}
		result, err = f.srv.DoActionQueryDomainDataSourceRequest(context.Background(), &request)
	default:
		err = status.Errorf(codes.InvalidArgument, "unsupported action type: %s", action.Type)
	}

	if err != nil {
		nlog.Warnf("DoAction %s fail: %v", action.Type, err)
		return err
	}

	return stream.Send(result)
}
