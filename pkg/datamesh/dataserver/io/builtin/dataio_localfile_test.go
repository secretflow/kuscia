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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const defaultLocalFSPath = "/tmp/var"

func TestNewBuiltinLocalFileIOChannel(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewBuiltinLocalFileIOChannel())
}

func initLocalFileDataIOTestRequestContext(t *testing.T, filename string, isQuery bool) *utils.DataMeshRequestContext {
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// init ok
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, filename)
	var msg protoreflect.ProtoMessage
	if isQuery {
		msg = &datamesh.CommandDomainDataQuery{
			DomaindataId: domainDataID,
		}
	} else {
		msg = &datamesh.CommandDomainDataUpdate{
			DomaindataId: domainDataID,
		}
	}
	ctx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, msg, common.DomainDataSourceTypeLocalFS)

	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	return ctx
}

func registLocalFileDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dsID string) {
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Localfs: &datamesh.LocalDataSourceInfo{
			Path: defaultLocalFSPath,
		}})
	assert.NoError(t, err)
	assert.NoError(t, paths.EnsurePath(defaultLocalFSPath, true))
	strConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, lfs)
	assert.NoError(t, err)

	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainDataSource{
		ObjectMeta: v1.ObjectMeta{
			Name: dsID,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Name: dsID,
			Type: "localfs",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func TestLocalFileIOChannel_Read_Invalidate(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinLocalFileIOChannel()

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	ctx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	}, common.DomainDataSourceTypeLocalFS)

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

	ctx.Query.ContentType = -1
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

	ctx.Query.ContentType = datamesh.ContentType_RAW

	channel := NewBuiltinLocalFileIOChannel()

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
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

	ctx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	}, common.DomainDataSourceTypeLocalFS)

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
	ctx.Update.ContentType = -1
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

	ctx.Update.ContentType = datamesh.ContentType_RAW

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
	assert.Equal(t, utils.BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
