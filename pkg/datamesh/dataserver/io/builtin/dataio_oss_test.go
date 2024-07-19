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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func initContextTestEnv(t *testing.T) *config.DataMeshConfig {
	conf := config.NewDefaultDataMeshConfig()

	assert.NotNil(t, conf)
	conf.KusciaClient = kusciafake.NewSimpleClientset()
	conf.KubeNamespace = "test-namespace"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf.DomainKey = privateKey

	return conf
}

func registDomainData(t *testing.T, conf *config.DataMeshConfig, dsID, pathName string) string {
	domainDataID := "data-" + uuid.New().String()
	_, err := conf.KusciaClient.KusciaV1alpha1().DomainDatas(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainData{
		ObjectMeta: v1.ObjectMeta{
			Name: domainDataID,
		},
		Spec: v1alpha1.DomainDataSpec{
			RelativeURI: pathName,
			Name:        domainDataID,
			Type:        "RAW",
			DataSource:  dsID,
			Author:      conf.KubeNamespace,
		},
	}, v1.CreateOptions{})
	assert.NoError(t, err)

	return domainDataID
}

type mockGrpcServerStream struct{}

func (*mockGrpcServerStream) Context() context.Context {
	return context.Background()
}
func (*mockGrpcServerStream) RecvMsg(m any) error          { return nil }
func (*mockGrpcServerStream) SendHeader(metadata.MD) error { return nil }
func (*mockGrpcServerStream) SendMsg(m any) error          { return nil }
func (*mockGrpcServerStream) SetHeader(metadata.MD) error  { return nil }
func (*mockGrpcServerStream) SetTrailer(metadata.MD)       {}

type mockDoGetServer struct {
	grpc.ServerStream
	dataList []*flight.FlightData
}

func (mgs *mockDoGetServer) Send(data *flight.FlightData) error {
	nlog.Debugf("mockDoGetServer.Send")
	mgs.dataList = append(mgs.dataList, deepCopy(data))
	return nil
}

type mockDoPutServer struct {
	grpc.ServerStream
	nextDataList []*flight.FlightData
}

func (mps *mockDoPutServer) Send(*flight.PutResult) error {
	return nil
}

func (mps *mockDoPutServer) Recv() (*flight.FlightData, error) {
	if len(mps.nextDataList) == 0 {
		return nil, io.EOF
	}
	nlog.Debugf("mockDoPutServer.Recv")
	data := mps.nextDataList[0]
	mps.nextDataList = mps.nextDataList[1:]
	return data, nil
}

func deepCopy(src *flight.FlightData) *flight.FlightData {
	var dest bytes.Buffer
	enc := gob.NewEncoder(&dest)
	dec := gob.NewDecoder(&dest)

	err := enc.Encode(src)
	if err != nil {
		return nil
	}

	var result *flight.FlightData
	err = dec.Decode(&result)
	if err != nil {
		return nil
	}

	return result
}

func registOssDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dsID string) {
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
			Name: dsID,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Name: dsID,
			Type: "oss",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func initOssDataIOTestRequestContext(t *testing.T, filename string, isQuery bool) (*config.DataMeshConfig, *utils.DataMeshRequestContext) {
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// init ok
	registOssDomainDataSource(t, conf, "oss-data-source")
	domainDataID := registDomainData(t, conf, "oss-data-source", filename)
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
	return conf, ctx
}

func updateOssDataSourceConfig(t *testing.T, conf *config.DataMeshConfig, ctx *utils.DataMeshRequestContext) (*s3mem.Backend, *httptest.Server) {
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

	ctx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	}, common.DomainDataSourceTypeOSS)

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
	ctx.Query.ContentType = -1
	assert.Error(t, channel.Read(context.Background(), ctx, nil))

	ctx.Query.ContentType = datamesh.ContentType_RAW

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
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

	ctx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "not-exists-domain-data",
		ContentType:  datamesh.ContentType_RAW,
	}, common.DomainDataSourceTypeOSS)

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
	ctx.Update.ContentType = -1
	assert.Error(t, channel.Write(context.Background(), ctx, nil))
}

func TestOssIOChannel_Write_Success(t *testing.T) {
	t.Parallel()
	filename := fmt.Sprintf("osstest-%s.txt", uuid.New().String())
	conf, ctx := initOssDataIOTestRequestContext(t, filename, false)

	channel := NewBuiltinOssIOChannel()

	_, ts := updateOssDataSourceConfig(t, conf, ctx)
	defer ts.Close()

	ctx.Update.ContentType = datamesh.ContentType_RAW

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
	assert.Equal(t, utils.BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
