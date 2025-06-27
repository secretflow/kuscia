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

package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
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

const defaultLocalFSPath = "/tmp/var"

type mockIOChannel struct {
	nextError error
}

func (mio *mockIOChannel) Read(ctx context.Context, rc *utils.DataMeshRequestContext, w *flight.Writer) error {
	return mio.nextError
}

func (mio *mockIOChannel) Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error {
	return mio.nextError
}

func (mio *mockIOChannel) GetEndpointURI() string {
	return ""
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

func registLocalFileDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dsID string) {
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Localfs: &datamesh.LocalDataSourceInfo{
			Path: defaultLocalFSPath,
		}})
	assert.NoError(t, err)
	if _, err = os.Stat(defaultLocalFSPath); os.IsNotExist(err) {
		err = os.MkdirAll(defaultLocalFSPath, 0755)
		assert.Nil(t, err)
	}
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

type mockDoGetServer struct {
	grpc.ServerStream

	dataList []*flight.FlightData
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

type mockDoActionServer struct {
	grpc.ServerStream
}

func (*mockDoActionServer) Send(*flight.Result) error {
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

func TestNewDataMeshFlightHandler(t *testing.T) {
	t.Parallel()
	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{
		{
			Mode:            string(config.ModeDirect),
			DataSourceTypes: []string{"odps"},
			Endpoint:        "127.0.0.1:10000",
		},
	})
	assert.NotNil(t, svr)

	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)
	assert.Greater(t, len(dm.customHandles), 4)
}

func TestGetFlightInf_FAILED(t *testing.T) {
	t.Parallel()
	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	// anyCmd.UnmarshalNew failed
	info, err := svr.GetFlightInfo(context.Background(), &flight.FlightDescriptor{})
	assert.Error(t, err)
	assert.Nil(t, info)

	// proto.Unmarshal failed
	info, err = svr.GetFlightInfo(context.Background(), &flight.FlightDescriptor{
		Cmd: []byte("xyz"),
	})
	assert.Error(t, err)
	assert.Nil(t, info)

	// NewDataMeshRequestContext failed
	desc, err := utils.DescForCommand(&datamesh.CommandGetDomainDataSchema{})
	assert.NoError(t, err)
	info, err = svr.GetFlightInfo(context.Background(), desc)
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestGetFlightInf_DataSource_Invalidate(t *testing.T) {
	t.Parallel()

	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	// datasource not registered
	domainDataID := registDomainData(t, conf, "test-datasouce", filename)
	desc, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	})
	assert.NoError(t, err)
	info, err := svr.GetFlightInfo(context.Background(), desc)
	assert.Error(t, err)
	assert.Nil(t, info)

	// registered datasource but type is invalidate
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{})
	assert.NoError(t, err)

	strConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, lfs)
	assert.NoError(t, err)

	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainDataSource{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-datasouce",
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Name: "test-datasouce",
			Type: "odps",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)

	info, err = svr.GetFlightInfo(context.Background(), desc)
	assert.Error(t, err) // not found channel
	assert.Nil(t, info)
}

func TestGetFlightInf_Success(t *testing.T) {
	t.Parallel()

	filename := fmt.Sprintf("localtest-%s.txt", uuid.New().String())

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, filename)
	desc, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	})
	assert.NoError(t, err)
	info, err := svr.GetFlightInfo(context.Background(), desc)
	assert.NoError(t, err)
	assert.NotNil(t, info)

	assert.Len(t, info.Endpoint, 1)
	assert.NotNil(t, info.Endpoint[0])
	assert.Len(t, info.Endpoint[0].Location, 1)
	assert.NotNil(t, info.Endpoint[0].Location[0])
	assert.NotEmpty(t, info.Endpoint[0].Location[0].Uri)
	assert.NotNil(t, info.Endpoint[0].Ticket)
	assert.NotEmpty(t, info.Endpoint[0].Ticket.Ticket)

	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)

}

func TestDoGet_InvalidateTicket(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})

	assert.Error(t, svr.DoGet(&flight.Ticket{
		Ticket: []byte("invalidate-ticket"),
	}, nil))
}

func TestDoGet_Success(t *testing.T) {
	t.Parallel()
	// Construct Flight Handler
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)
	// Mock datasource and domain data
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	fileName := "Handler_TestDoGet_Success.file"
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, fileName)
	// init test file
	filepath := path.Join(defaultLocalFSPath, fileName)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0777)
	assert.NoError(t, err)
	assert.NotNil(t, file)
	_, err = file.WriteString("hello world!")
	assert.NoError(t, err)
	assert.NoError(t, file.Close())
	defer os.Remove(filepath)
	// Get FlightInfo
	desc, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
		ContentType:  datamesh.ContentType_RAW,
	})
	fl, err := svr.GetFlightInfo(context.Background(), desc)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
	// Do Get
	err = svr.DoGet(fl.Endpoint[0].GetTicket(), &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	})
	assert.Nil(t, err)
}

func TestDoPut_Failed(t *testing.T) {
	t.Parallel()
	// Construct Flight Service
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	assert.NotNil(t, svr)
	// Mock datasource and domain data
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	fileName := "TestDoPut_Failed_NotExist.file"
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, fileName)

	// Get FlightInfo
	desc, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
		ContentType:  datamesh.ContentType_RAW,
	})
	assert.Nil(t, err)
	assert.NotNil(t, desc)
	fl, err := svr.GetFlightInfo(context.Background(), desc)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
	// construct input stream
	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	w := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, w)
	w.Close()
	assert.NotEmpty(t, mgs.dataList)
	assert.Len(t, mgs.dataList, 1)
	dataList := mgs.dataList
	dataList[0].FlightDescriptor = &flight.FlightDescriptor{
		Cmd: []byte("Not-Exist-Ticket"),
	}
	// do PUT
	err = svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	})

	assert.NotNil(t, err)
}

func TestDoPut_Success(t *testing.T) {
	t.Parallel()
	// Construct Flight Service
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	assert.NotNil(t, svr)
	// Mock datasource and domain data
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	fileName := "TestHanlderDoPut_Success.file"
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, fileName)

	// Get FlightInfo
	desc, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
		ContentType:  datamesh.ContentType_RAW,
	})
	assert.Nil(t, err)
	assert.NotNil(t, desc)
	fl, err := svr.GetFlightInfo(context.Background(), desc)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
	// construct input stream
	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	w := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, w)
	w.Close()
	assert.NotEmpty(t, mgs.dataList)
	assert.Len(t, mgs.dataList, 1)
	dataList := mgs.dataList
	dataList[0].FlightDescriptor = &flight.FlightDescriptor{
		Cmd: fl.Endpoint[0].GetTicket().Ticket,
	}
	// do PUT
	err = svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	})
	defer os.Remove(path.Join(defaultLocalFSPath, fileName))
	assert.Nil(t, err)
}

func TestDoAction_Failed(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	assert.Error(t, svr.DoAction(&flight.Action{
		Type: "not-exists",
	}, nil))

	// recover
	assert.Error(t, svr.DoAction(&flight.Action{
		Type: "ActionCreateDomainDataRequest",
	}, nil))
}

func TestDoAction_Success(t *testing.T) {
	t.Parallel()

	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	body, _ := proto.Marshal(&datamesh.CreateDomainDataRequest{
		DomaindataId: "test-domain-data",
	})

	// datasource not registered
	assert.Error(t, svr.DoAction(&flight.Action{
		Type: "ActionCreateDomainDataRequest",
		Body: body,
	}, nil))

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)

	body, _ = proto.Marshal(&datamesh.CreateDomainDataRequest{
		DomaindataId: "test-domain-data",
		DatasourceId: common.DefaultDataSourceID,
	})

	assert.NoError(t, svr.DoAction(&flight.Action{
		Type: "ActionCreateDomainDataRequest",
		Body: body,
	}, &mockDoActionServer{}))
}

func TestGetFlightInfo_recover(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})
	assert.NotNil(t, svr)
	info, err := svr.GetFlightInfo(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestDoGet_recover(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	assert.Error(t, svr.DoGet(nil, nil))
}

func TestDoPut_recover(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.DataProxyConfig{})
	assert.NotNil(t, svr)

	assert.Error(t, svr.DoPut(nil))
}
