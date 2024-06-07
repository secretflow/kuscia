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

package dmflight

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func TestNewBuiltinLocalFileIOChannel(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewBuiltinLocalFileIOChannel())
}

func initLocalFileDataIOTestRequestContext(t *testing.T, filename string, isQuery bool) *DataMeshRequestContext {
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// init ok
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataId := registDomainData(t, conf, common.DefaultDataSourceID, filename)
	var msg protoreflect.ProtoMessage
	if isQuery {
		msg = &datamesh.CommandDomainDataQuery{
			DomaindataId: domainDataId,
		}
	} else {
		msg = &datamesh.CommandDomainDataUpdate{
			DomaindataId: domainDataId,
		}
	}
	ctx, err := NewDataMeshRequestContext(domainDataService, datasourceService, msg)

	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	return ctx
}

func TestLocalFileIOChannel_Read_Invalidate(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinLocalFileIOChannel()

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	ctx, err := NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	assert.Error(t, channel.Read(context.Background(), ctx, nil))
}

func TestLocalFileIOChannel_Read_OpenFileFailed(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())
	ctx := initLocalFileDataIOTestRequestContext(t, filename, true)

	channel := NewBuiltinLocalFileIOChannel()
	// file not exists
	assert.Error(t, channel.Read(context.Background(), ctx, nil))

	// invalidate content-type
	// write test file
	dd, ds, err := ctx.GetDomainDataAndSource(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.NotNil(t, dd)

	filepath := path.Join(ds.Info.Localfs.Path, dd.RelativeUri)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0777)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	_, err = file.WriteString("hello world!")
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	ctx.query.ContentType = -1
	assert.Error(t, channel.Read(context.Background(), ctx, nil))
	assert.NoError(t, os.Remove(filepath))
}

func TestLocalFileIOChannel_Read_Success(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())

	ctx := initLocalFileDataIOTestRequestContext(t, filename, true)
	dd, ds, err := ctx.GetDomainDataAndSource(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.NotNil(t, dd)

	// init test file
	filepath := path.Join(ds.Info.Localfs.Path, dd.RelativeUri)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0777)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	_, err = file.WriteString("hello world!")
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	ctx.query.ContentType = datamesh.ContentType_RAW

	channel := NewBuiltinLocalFileIOChannel()

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, writer)

	// write success
	assert.NoError(t, channel.Read(context.Background(), ctx, writer))

	assert.NoError(t, os.Remove(filepath))
}

func TestLocalFileIOChannel_Write_Invalidate(t *testing.T) {
	channel := NewBuiltinLocalFileIOChannel()

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	ctx, err := NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	assert.Error(t, channel.Write(context.Background(), ctx, nil))
}

func TestLocalFileIOChannel_Write_OpenFileFailed(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())
	ctx := initLocalFileDataIOTestRequestContext(t, filename, false)

	channel := NewBuiltinLocalFileIOChannel()
	// file exists

	dd, ds, err := ctx.GetDomainDataAndSource(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.NotNil(t, dd)

	filepath := path.Join(ds.Info.Localfs.Path, dd.RelativeUri)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0777)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	_, err = file.WriteString("hello world!")
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	assert.Error(t, channel.Write(context.Background(), ctx, nil))

	assert.NoError(t, os.Remove(filepath))

	// invalidate content-type
	ctx.update.ContentType = -1
	assert.Error(t, channel.Write(context.Background(), ctx, nil))

}

func TestLocalFileIOChannel_Write_Success(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())

	ctx := initLocalFileDataIOTestRequestContext(t, filename, false)
	dd, ds, err := ctx.GetDomainDataAndSource(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.NotNil(t, dd)

	ctx.update.ContentType = datamesh.ContentType_RAW

	channel := NewBuiltinLocalFileIOChannel()

	inputs := getFlightData(t, [][]byte{})

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	// write without
	assert.NoError(t, channel.Write(context.Background(), ctx, reader))

	filepath := path.Join(ds.Info.Localfs.Path, dd.RelativeUri)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0777)
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.NoError(t, file.Close())

	assert.NoError(t, os.Remove(filepath))
}

func TestLocalfileIOChannel_Endpoint(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinLocalFileIOChannel()
	assert.Equal(t, BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
