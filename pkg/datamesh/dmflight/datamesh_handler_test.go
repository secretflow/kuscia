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
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

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
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type mockIOChannel struct {
	nextError error
}

func (mio *mockIOChannel) Read(ctx context.Context, rc *DataMeshRequestContext, w *flight.Writer) error {
	return mio.nextError
}

func (mio *mockIOChannel) Write(ctx context.Context, rc *DataMeshRequestContext, stream *flight.Reader) error {
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
	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{
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

	assert.Equal(t, 3, len(dm.iochannels))
	assert.NotNil(t, dm.iochannels["localfs"])
	assert.NotNil(t, dm.iochannels["oss"])
	assert.NotNil(t, dm.iochannels["odps"])
	assert.NotNil(t, dm.cmds)
}

func TestGetFlightInf_FAILED(t *testing.T) {
	t.Parallel()
	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
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
	desc, err := DescForCommand(&datamesh.CommandGetDomainDataSchema{})
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

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)

	// datasource not registed
	domainDataId := registDomainData(t, conf, "test-datasouce", filename)
	desc, err := DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataId,
	})
	assert.NoError(t, err)
	info, err := svr.GetFlightInfo(context.Background(), desc)
	assert.Error(t, err)
	assert.Nil(t, info)

	// reigst datasource but type is invalidate
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

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	domainDataId := registDomainData(t, conf, common.DefaultDataSourceID, filename)
	desc, err := DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataId,
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

	itm, ok := dm.cmds.Get(string(info.Endpoint[0].Ticket.Ticket))
	assert.True(t, ok)
	assert.NotNil(t, itm)

	rc := itm.(*DataMeshRequestContext)
	assert.NotNil(t, rc)
	assert.NotNil(t, rc.query)
	assert.NotNil(t, rc.io)
	assert.Equal(t, domainDataId, rc.query.DomaindataId)
}

func TestDoGet_InvalidateTicket(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})

	assert.Error(t, svr.DoGet(&flight.Ticket{
		Ticket: []byte("invalidate-ticket"),
	}, nil))
}

func TestDoGet_Success(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)

	assert.NoError(t, dm.cmds.Add("test-ticket", &DataMeshRequestContext{
		query: &datamesh.CommandDomainDataQuery{
			ContentType: datamesh.ContentType_RAW,
		},
		io: &mockIOChannel{},
	}, time.Minute))

	assert.NoError(t, svr.DoGet(&flight.Ticket{
		Ticket: []byte("test-ticket"),
	}, &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}))
}

func TestDoPut_Failed(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)

	// create reader failed
	assert.Error(t, svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: []*flight.FlightData{},
	}))

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	w := flight.NewRecordWriter(mgs, ipc.WithSchema(GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, w)
	w.Close()
	assert.NotEmpty(t, mgs.dataList)
	nlog.Infof("writen data count=%d", len(mgs.dataList))
	// no desc
	assert.Error(t, svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	}))

	dataList := mgs.dataList
	dataList[0].FlightDescriptor = &flight.FlightDescriptor{
		Cmd: []byte("not-exists-ticket"),
	}

	assert.Error(t, svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	}))
}

func TestDoPut_Success(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	dm := svr.(*datameshFlightHandler)
	assert.NotNil(t, dm)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	w := flight.NewRecordWriter(mgs, ipc.WithSchema(GenerateBinaryDataArrowSchema()))
	assert.NotNil(t, w)
	w.Close()
	assert.NotEmpty(t, mgs.dataList)

	assert.Len(t, mgs.dataList, 1)

	dataList := mgs.dataList
	dataList[0].FlightDescriptor = &flight.FlightDescriptor{
		Cmd: []byte("test-ticket"),
	}

	assert.NoError(t, dm.cmds.Add("test-ticket", &DataMeshRequestContext{
		query: &datamesh.CommandDomainDataQuery{
			ContentType: datamesh.ContentType_RAW,
		},
		io: &mockIOChannel{},
	}, time.Minute))

	assert.NoError(t, svr.DoPut(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	}))
}

func TestDoAction_Failed(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
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

	svr := NewDataMeshFlightHandler(domainDataService, datasourceService, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)

	body, _ := proto.Marshal(&datamesh.CreateDomainDataRequest{
		DomaindataId: "test-domain-data",
	})

	// datasource not registed
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

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)
	info, err := svr.GetFlightInfo(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestDoGet_recover(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)

	assert.Error(t, svr.DoGet(nil, nil))
}

func TestDoPut_recover(t *testing.T) {
	t.Parallel()

	svr := NewDataMeshFlightHandler(nil, nil, []config.ExternalDataProxyConfig{})
	assert.NotNil(t, svr)

	assert.Error(t, svr.DoPut(nil))
}
