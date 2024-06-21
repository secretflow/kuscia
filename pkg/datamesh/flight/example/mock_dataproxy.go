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

//nolint:dupl
package example

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/csv"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type MockDataProxy struct {
	Addr          string
	ticket2Query  map[string]*datamesh.CommandDataMeshQuery
	ticket2Update map[string]*datamesh.CommandDataMeshUpdate
	flight.BaseFlightServer
}

func NewMockDataProxy(addr string) *MockDataProxy {
	return &MockDataProxy{
		Addr:             addr,
		ticket2Query:     make(map[string]*datamesh.CommandDataMeshQuery),
		ticket2Update:    make(map[string]*datamesh.CommandDataMeshUpdate),
		BaseFlightServer: flight.BaseFlightServer{},
	}
}

func (m *MockDataProxy) Start() error {
	server := flight.NewServerWithMiddleware(nil)
	server.Init(m.Addr)
	server.RegisterFlightService(m)

	nlog.Infof("start data proxy at:%s", m.Addr)
	return server.Serve()
}

func (m *MockDataProxy) GetFlightInfo(ctx context.Context, request *flight.FlightDescriptor) (*flight.FlightInfo,
	error) {
	var (
		anyCmd anypb.Any
		msg    proto.Message
		err    error
		ticket *flight.Ticket
	)

	if err = proto.Unmarshal(request.Cmd, &anyCmd); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unable to parse command: %s", err.Error())
	}

	if msg, err = anyCmd.UnmarshalNew(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal Any to a command type: %s", err.Error())
	}

	switch cmd := msg.(type) {
	case *datamesh.CommandDataMeshQuery:
		ticket, err = m.buildMockTicket(cmd)
	case *datamesh.CommandDataMeshUpdate:
		ticket, err = m.buildMockUpdateTicket(cmd)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid command")
	}

	if err != nil {
		return nil, err
	}

	return m.buildMockFlightInfo(ticket)
}

func (m *MockDataProxy) DoGet(tkt *flight.Ticket, fs flight.FlightService_DoGetServer) error {
	var (
		anyCmd anypb.Any
		msg    proto.Message
		err    error
	)
	if err = proto.Unmarshal(tkt.Ticket, &anyCmd); err != nil {
		return status.Errorf(codes.Internal, "ticket unmarshall fail")
	}
	if msg, err = anyCmd.UnmarshalNew(); err != nil {
		return status.Errorf(codes.Internal, "ticket unmarshallNew fail")
	}

	switch cmd := msg.(type) {
	case *datamesh.TicketDomainDataQuery:
		return m.getDataByTicket(cmd.DomaindataHandle, fs)
	default:
		return status.Errorf(codes.Internal, "unknown ticket")
	}
}

func (m *MockDataProxy) DoPut(stream flight.FlightService_DoPutServer) error {
	var (
		anyCmd anypb.Any
		err    error
		query  datamesh.TicketDomainDataQuery
		rdr    *flight.Reader
	)

	rdr, err = flight.NewRecordReader(stream)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	// creating the reader should have gotten the first message which would
	// have the schema, which should have a populated flight descriptor
	desc := rdr.LatestFlightDescriptor()
	if err = proto.Unmarshal(desc.Cmd, &anyCmd); err != nil {
		return status.Errorf(codes.Internal, "ticket for Put unmarshall fail")
	}

	if err = anyCmd.UnmarshalTo(&query); err != nil {
		return status.Errorf(codes.Internal, "ticket for Put unmarshallNew fail")
	}

	ticketID := query.DomaindataHandle
	update, _ := m.ticket2Update[ticketID]
	if update == nil {
		return status.Errorf(codes.Internal, "ticket for put is invalid:%s", ticketID)
	}

	if update.Datasource.Type != "localfs" {
		return status.Errorf(codes.Internal, "mock dp just support put to localfs")
	}

	path := strings.TrimRight(update.Datasource.Info.Localfs.Path, "/") + "/" + strings.TrimLeft(update.Domaindata.
		RelativeUri, "/")
	nlog.Infof("path of put request is :%s", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return status.Errorf(codes.Internal, "open file:%s fail, %v", path, err)
	}
	w := csv.NewWriter(file, rdr.Schema(), csv.WithComma(','))
	defer file.Close()
	for rdr.Next() {
		rec := rdr.Record()
		rec.Retain()
		w.Write(rec)
		w.Flush()
		if len(rdr.LatestAppMetadata()) > 0 {
			stream.Send(&flight.PutResult{AppMetadata: rdr.LatestAppMetadata()})
		}
	}
	m.ticket2Update[ticketID] = nil
	return nil
}

func (m *MockDataProxy) getDataByTicket(ticketID string, fs flight.FlightService_DoGetServer) error {
	//recs := Records[ticketID]

	query, ok := m.ticket2Query[ticketID]
	if !ok || query == nil {
		status.Errorf(codes.Internal, fmt.Sprintf("invalid ticket:%s", ticketID))
	}

	fields := make([]arrow.Field, 0)
	for i, column := range query.Domaindata.Columns {
		colType := common.Convert2ArrowColumnType(column.Type)
		if colType == nil {
			return status.Errorf(codes.Internal, "invalid column(%s）with type(%s)", column.Name, column.Type)
		}
		fields = append(fields, arrow.Field{
			Name:     column.Name,
			Type:     colType,
			Nullable: !column.NotNullable,
		})
		nlog.Debugf("columns[%d].Nullable is :%v", i, !column.NotNullable)
	}
	schema := arrow.NewSchema(fields, nil)

	if query.Datasource.Type != "localfs" {
		return status.Errorf(codes.Internal, "mock dp just support put to localfs")
	}
	path := query.Datasource.Info.Localfs.Path + query.Domaindata.RelativeUri
	raw, err := os.ReadFile(path)
	if err != nil {
		return status.Errorf(codes.Internal, "file %s open fail:%v", path, err)
	}

	reader := csv.NewReader(bytes.NewReader(raw), schema, csv.WithComma(','), csv.WithNullReader(true))
	recs := make([]arrow.Record, 0)

	for reader.Next() {
		rec := reader.Record()
		rec.Retain()
		recs = append(recs, rec)
	}

	nlog.Infof("read %d lines of %s", len(recs), path)

	if query.Query.ContentType == datamesh.ContentType_RAW {
		dummySchema := arrow.NewSchema([]arrow.Field{
			{
				Name:     "dummy",
				Type:     arrow.BinaryTypes.Binary,
				Nullable: false,
			},
		}, nil)
		w := flight.NewRecordWriter(fs, ipc.WithSchema(dummySchema))

		mask := []bool{
			false,
		}
		var chunks [][]arrow.Array
		defer func() {
			for _, chunk := range chunks {
				for _, col := range chunk {
					col.Release()
				}
			}
			w.Close()
		}()

		mem := memory.NewGoAllocator()
		for idx, r := range recs {
			f := new(bytes.Buffer)
			csvWriter := csv.NewWriter(f, schema)
			csvWriter.Write(r)
			csvWriter.Flush()

			chunk := []arrow.Array{
				arrayOf(mem, [][]byte{f.Bytes()}, mask),
			}
			chunks = append(chunks, chunk)
			newR := array.NewRecord(dummySchema, chunk, -1)
			w.WriteWithAppMetadata(newR, []byte(fmt.Sprintf("%d_%s", idx, ticketID)) /*metadata*/)
		}
	} else {
		w := flight.NewRecordWriter(fs, ipc.WithSchema(recs[0].Schema()))
		defer w.Close()
		for idx, r := range recs {
			w.WriteWithAppMetadata(r, []byte(fmt.Sprintf("%d_%s", idx, ticketID)) /*metadata*/)
		}
	}

	m.ticket2Query[ticketID] = nil
	return nil
}

func (m *MockDataProxy) buildMockFlightInfo(ticket *flight.Ticket) (*flight.FlightInfo, error) {
	return &flight.FlightInfo{
		Schema: nil,
		FlightDescriptor: &flight.FlightDescriptor{
			Type: flight.DescriptorCMD,
			Cmd:  []byte{},
			Path: []string{},
		},
		Endpoint: []*flight.FlightEndpoint{
			{
				Ticket: ticket,
				Location: []*flight.Location{
					{
						Uri: fmt.Sprintf("grpc+tcp://%s", m.Addr),
					},
				},
				ExpirationTime: nil,
			},
		},
	}, nil
}

func (m *MockDataProxy) buildMockTicket(query *datamesh.CommandDataMeshQuery) (*flight.Ticket, error) {
	ticket := &datamesh.TicketDomainDataQuery{
		DomaindataHandle: query.Domaindata.DomaindataId,
	}

	bakQuery, ok := proto.Clone(query).(*datamesh.CommandDataMeshQuery)
	if !ok {
		return nil, status.Error(codes.Internal, "clone CommandDataMeshQuery fail ")
	}

	m.ticket2Query[query.Domaindata.DomaindataId] = bakQuery

	var (
		anyCmd anypb.Any
		err    error
		body   []byte
	)

	if err = anyCmd.MarshalFrom(ticket); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("%v: unable to marshal final response", err))
	}
	if body, err = proto.Marshal(&anyCmd); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &flight.Ticket{
		Ticket: body,
	}, nil
}

func (m *MockDataProxy) buildMockUpdateTicket(query *datamesh.CommandDataMeshUpdate) (*flight.Ticket, error) {
	ticket := &datamesh.TicketDomainDataQuery{
		DomaindataHandle: query.Domaindata.DomaindataId,
	}

	bakQuery, ok := proto.Clone(query).(*datamesh.CommandDataMeshUpdate)
	if !ok {
		return nil, status.Error(codes.Internal, "clone CommandDataMeshQuery fail ")
	}

	m.ticket2Update[query.Domaindata.DomaindataId] = bakQuery

	var (
		anyCmd anypb.Any
		err    error
		body   []byte
	)

	if err = anyCmd.MarshalFrom(ticket); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("%v: unable to marshal final response", err))
	}
	if body, err = proto.Marshal(&anyCmd); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &flight.Ticket{
		Ticket: body,
	}, nil
}
