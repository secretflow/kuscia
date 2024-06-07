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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func registOssDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dsId string) {
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Oss: &datamesh.OssDataSourceInfo{
			Endpoint: "",
			Bucket:   "test",
		}})
	assert.NoError(t, err)

	strConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, lfs)
	assert.NoError(t, err)

	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainDataSource{
		ObjectMeta: v1.ObjectMeta{
			Name: dsId,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Name: dsId,
			Type: "oss",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func initOssDataIOTestRequestContext(t *testing.T, filename string, isQuery bool) (*config.DataMeshConfig, *DataMeshRequestContext) {
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// init ok
	registOssDomainDataSource(t, conf, "oss-data-source")
	domainDataId := registDomainData(t, conf, "oss-data-source", filename)
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
	return conf, ctx
}

func updateOssDataSourceConfig(t *testing.T, conf *config.DataMeshConfig, ctx *DataMeshRequestContext) (*s3mem.Backend, *httptest.Server) {
	// oss file not exists
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())

	// change datasource config
	ds, err := conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Get(context.Background(), "oss-data-source", v1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.NotNil(t, ds.Spec)

	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Oss: &datamesh.OssDataSourceInfo{
			Endpoint:        ts.URL,
			Bucket:          "test",
			Prefix:          "",
			StorageType:     "minio",
			AccessKeyId:     "xz",
			AccessKeySecret: "mmmm",
		}})
	assert.NoError(t, err)

	strConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, lfs)
	assert.NoError(t, err)

	ds.Spec.Data = map[string]string{
		"encryptedInfo": strConfig,
	}
	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Update(context.Background(), ds, v1.UpdateOptions{})
	assert.NoError(t, err)

	return backend, ts
}

func TestNewBuiltinOssIOChannel(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewBuiltinOssIOChannel())
}

func TestOssIOChannel_Read_DomainData_Invalidate(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinOssIOChannel()

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

func TestOssIOChannel_Read_OssQueryFailed(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("osstest-%s.txt", uuid.New().String())
	conf, ctx := initOssDataIOTestRequestContext(t, filename, true)

	channel := NewBuiltinOssIOChannel()
	// create oss session failed
	assert.Error(t, channel.Read(context.Background(), ctx, nil))

	_, ts := updateOssDataSourceConfig(t, conf, ctx)
	defer ts.Close()

	// oss file not exists
	assert.Error(t, channel.Read(context.Background(), ctx, nil))
}

func TestOssIOChannel_Read_Success(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("osstest-%s.txt", uuid.New().String())
	conf, ctx := initOssDataIOTestRequestContext(t, filename, true)

	channel := NewBuiltinOssIOChannel()
	// create oss session failed
	assert.Error(t, channel.Read(context.Background(), ctx, nil))

	backend, ts := updateOssDataSourceConfig(t, conf, ctx)
	defer ts.Close()

	assert.NoError(t, backend.CreateBucket("test"))
	_, err := backend.PutObject("test", filename, map[string]string{}, bytes.NewReader([]byte("abcdefg")), 7)
	assert.NoError(t, err)

	// invalidate type
	ctx.query.ContentType = -1
	assert.Error(t, channel.Read(context.Background(), ctx, nil))

	ctx.query.ContentType = datamesh.ContentType_RAW

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, writer)

	// write success
	assert.NoError(t, channel.Read(context.Background(), ctx, writer))
	assert.NotEmpty(t, mgs.dataList)
}

func TestOssIOChannel_Write_DomainData_Invalidate(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinOssIOChannel()

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

func TestOssIOChannel_Write_RemoteOSSFailed(t *testing.T) {
	t.Parallel()

	filename := fmt.Sprintf("osstest-%s.txt", uuid.New().String())
	conf, ctx := initOssDataIOTestRequestContext(t, filename, false)

	channel := NewBuiltinOssIOChannel()

	_, ts := updateOssDataSourceConfig(t, conf, ctx)
	defer ts.Close()

	// invalidate content-type
	ctx.update.ContentType = -1
	assert.Error(t, channel.Write(context.Background(), ctx, nil))
}

func TestOssIOChannel_Write_Success(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("osstest-%s.txt", uuid.New().String())
	conf, ctx := initOssDataIOTestRequestContext(t, filename, false)

	channel := NewBuiltinOssIOChannel()

	_, ts := updateOssDataSourceConfig(t, conf, ctx)
	defer ts.Close()

	ctx.update.ContentType = datamesh.ContentType_RAW

	inputs := getFlightData(t, [][]byte{})

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	// write without
	assert.NoError(t, channel.Write(context.Background(), ctx, reader))
}

func TestOssIOChannel_Endpoint(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinOssIOChannel()
	assert.Equal(t, BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
