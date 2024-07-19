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
	"testing"

	"github.com/secretflow/kuscia/pkg/datamesh/config"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type MockFlightServer struct {
	flight.BaseFlightServer
}

func (m *MockFlightServer) GetFlightInfo(ctx context.Context, request *flight.FlightDescriptor) (*flight.FlightInfo,
	error) {
	info := &flight.FlightInfo{
		Schema:           nil,
		FlightDescriptor: nil,
		Endpoint: []*flight.FlightEndpoint{
			{
				Ticket: &flight.Ticket{
					Ticket: []byte("test-ticket"),
				},
				Location: []*flight.Location{
					{
						Uri: "dataproxy.DomainDataUnitTestNamespace.svc",
					},
				},
				ExpirationTime: nil,
			},
		},
	}
	return info, nil
}

func TestDataProxyClient(t *testing.T) {
	server := flight.NewServerWithMiddleware(nil)
	server.Init("localhost:0")
	srv := &MockFlightServer{}
	server.RegisterFlightService(srv)

	go server.Serve()
	defer server.Shutdown()

	conf := &config.DataProxyConfig{
		Endpoint: server.Addr().String(),
	}
	client, err := NewDataProxyClient(conf)
	assert.Nil(t, err)

	query := &datamesh.CommandDataMeshQuery{
		Query: &datamesh.CommandDomainDataQuery{
			DomaindataId: "test-data-id",
		},
	}
	info, err := client.GetFlightInfoDataMeshQuery(context.Background(), query)
	assert.Nil(t, err)
	ticket := string(info.Endpoint[0].Ticket.Ticket)
	assert.Equal(t, ticket, "test-ticket")
}
