// Copyright 2025 Ant Group Co., Ltd.
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

package main

import (
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockIDomainDataSourceService struct {
	mock.Mock
}

type MockFlightClientWrapper struct {
	mock.Mock
}

func (m *MockFlightClientWrapper) DoAction(ctx context.Context, action *flight.Action) (flight.FlightService_DoActionClient, error) {
	args := m.Called(ctx, action)
	return args.Get(0).(flight.FlightService_DoActionClient), args.Error(1)
}

func (m *MockFlightClientWrapper) GetFlightInfo(ctx context.Context, desc *flight.FlightDescriptor) (*flight.FlightInfo, error) {
	args := m.Called(ctx, desc)
	return args.Get(0).(*flight.FlightInfo), args.Error(1)
}

func (m *MockFlightClientWrapper) DoGet(ctx context.Context, ticket *flight.Ticket) (flight.FlightService_DoGetClient, error) {
	args := m.Called(ctx, ticket)
	return args.Get(0).(flight.FlightService_DoGetClient), args.Error(1)
}

func (m *MockFlightClientWrapper) DoPut(ctx context.Context) (flight.FlightService_DoPutClient, error) {
	args := m.Called(ctx)
	return args.Get(0).(flight.FlightService_DoPutClient), args.Error(1)
}

func (m *MockFlightClientWrapper) Close() error {
	args := m.Called()
	return args.Error(0)
}


func TestCreateFlightClient_Insecure(t *testing.T) {
	
	client, err := createFlightClient("grpc://localhost:8080", nil)
	
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client.Close()
}

func TestCreateClientCertificate(t *testing.T) {
	
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	
	config, err := createClientCertificate()

	
	assert.NoError(t, err)
	assert.NotNil(t, config)

	
	assert.FileExists(t, config.serverCertFile)
	assert.FileExists(t, config.serverKeyFile)
	assert.FileExists(t, config.clientCertFile)
	assert.FileExists(t, config.clientKeyFile)
	assert.FileExists(t, config.caFile)
}


type MockDoActionClient struct {
	mock.Mock
	flight.FlightService_DoActionClient
}

func (m *MockDoActionClient) Recv() (*flight.Result, error) {
	args := m.Called()
	return args.Get(0).(*flight.Result), args.Error(1)
}

func (m *MockDoActionClient) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}


type MockDoPutClient struct {
	mock.Mock
	flight.FlightService_DoPutClient
}

func (m *MockDoPutClient) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockDoPutClient) Recv() (*flight.PutResult, error) {
	args := m.Called()
	return args.Get(0).(*flight.PutResult), args.Error(1)
}

func (m *MockDoPutClient) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}
