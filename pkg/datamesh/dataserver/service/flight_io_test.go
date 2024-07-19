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

package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/gob"
	"encoding/json"
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
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const defaultLocalFSPath = "/tmp/var"

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

func TestNewFlightService(t *testing.T) {
	t.Parallel()
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)
}

func TestGetFlightInfo(t *testing.T) {
	t.Parallel()
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "filename")
	var msg protoreflect.ProtoMessage
	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	fl, err := fs.GetFlightInfo(context.Background(), msg)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
}

func TestFlightDoGet_Success(t *testing.T) {
	t.Parallel()
	// Construct Flight Service
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)
	// Mock datasource and domain data
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	fileName := "TestFlightDoGet_Success.file"
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
	// Get Flight Info
	msg := &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
		ContentType:  datamesh.ContentType_RAW,
	}
	fl, err := fs.GetFlightInfo(context.Background(), msg)
	assert.Nil(t, err)
	assert.NotNil(t, fl)
	// Do Get
	err = fs.DoGet(fl.Endpoint[0].GetTicket(), &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	})
	assert.Nil(t, err)
}

func TestFlightDoGet_NotExist(t *testing.T) {
	t.Parallel()
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "TestFlightDoGet_NotExist.file")
	var msg protoreflect.ProtoMessage
	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	fl, err := fs.GetFlightInfo(context.Background(), msg)
	assert.Nil(t, err)
	assert.NotNil(t, fl)

	err = fs.DoGet(fl.Endpoint[0].GetTicket(), &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	})
	assert.NotNil(t, err)
}

func TestFlightDoPut_Success(t *testing.T) {
	t.Parallel()
	// Construct Flight Service
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)
	// Mock datasource and domain data
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	fileName := "TestFlightDoPut_Success.output"
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, fileName)
	// Get FlightInfo
	var msg protoreflect.ProtoMessage
	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}
	fl, err := fs.GetFlightInfo(context.Background(), msg)
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
	err = fs.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	})
	defer os.Remove(path.Join(defaultLocalFSPath, fileName))
	assert.Nil(t, err)
}

func TestFlightDoPut_NotExist(t *testing.T) {
	t.Parallel()
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)
	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	fs := NewFlightIO(domainDataService, datasourceService, []config.DataProxyConfig{{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}})
	assert.NotNil(t, fs)
	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "TestFlightDoPut_NotExist.output")
	var msg protoreflect.ProtoMessage
	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}
	fl, err := fs.GetFlightInfo(context.Background(), msg)
	assert.Nil(t, err)
	assert.NotNil(t, fl)

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
	err = fs.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	})
	assert.NotNil(t, err)
}
