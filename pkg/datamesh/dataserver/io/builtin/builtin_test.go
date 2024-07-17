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

package builtin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func TestNewIOServer(t *testing.T) {
	ioServer := NewIOServer()
	assert.NotNil(t, ioServer, "TestNewIOServer")
}

func TestGetFlightInfo(t *testing.T) {

	ioServer := NewIOServer()
	assert.NotNil(t, ioServer)

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "filename")

	var msg protoreflect.ProtoMessage

	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	reqCtx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, msg, common.DomainDataSourceTypeLocalFS)
	assert.Nil(t, err)
	fl, err := ioServer.GetFlightInfo(context.Background(), reqCtx)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
}

func TestDoGet_NotExist(t *testing.T) {

	ioServer := NewIOServer()
	assert.NotNil(t, ioServer)

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "not-exist")
	var msg protoreflect.ProtoMessage

	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	reqCtx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, msg)
	fl, err := ioServer.GetFlightInfo(context.Background(), reqCtx)

	assert.Nil(t, err)
	assert.NotNil(t, fl)

	err = ioServer.DoGet(fl.Endpoint[0].GetTicket(), &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	})
	assert.NotNil(t, err)
}

func TestDoPut_NotExist(t *testing.T) {

	ioServer := NewIOServer()
	assert.NotNil(t, ioServer)

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "filename")
	var msg protoreflect.ProtoMessage

	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	reqCtx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, msg)
	fl, err := ioServer.GetFlightInfo(context.Background(), reqCtx)

	assert.Nil(t, err)
	assert.NotNil(t, fl)

	err = ioServer.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
	})
	assert.NotNil(t, err)
}
