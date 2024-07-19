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

package commands

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path"
	"testing"
	"time"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const datameshTestNamespace = "datamesh-test"

func newDataMeshTestConfig() *config.DataMeshConfig {
	conf := config.NewDefaultDataMeshConfig()

	conf.KubeNamespace = datameshTestNamespace
	conf.KusciaClient = kusciafake.NewSimpleClientset()
	conf.DisableTLS = true
	conf.ListenAddr = "127.0.0.1"
	conf.GRPCPort, _ = network.BuiltinPortAllocator.Next()
	conf.HTTPPort, _ = network.BuiltinPortAllocator.Next()
	conf.RootDir = "/tmp"

	conf.DomainKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	return conf
}

func isTCPPortOpened(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func TestFlightServerStartup(t *testing.T) {
	t.Parallel()
	conf := newDataMeshTestConfig()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := Run(ctx, conf, conf.KusciaClient)
		assert.NoError(t, err)
	}()

	grpcAddress := fmt.Sprintf("127.0.0.1:%d", conf.GRPCPort)

	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second, func() (bool, error) {
		return isTCPPortOpened(grpcAddress, time.Millisecond*20), nil
	}))

	toc, c1 := context.WithTimeout(ctx, time.Millisecond*100)
	defer c1()

	conn, err := grpc.DialContext(toc, grpcAddress, grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	// stop all ports
	cancel()
}

func TestFlightServer_GetFlightInfo_Query_DOGET(t *testing.T) {
	t.Parallel()
	conf := newDataMeshTestConfig()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := Run(ctx, conf, conf.KusciaClient)
		assert.NoError(t, err)
	}()

	grpcAddress := fmt.Sprintf("127.0.0.1:%d", conf.GRPCPort)

	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second, func() (bool, error) {
		return isTCPPortOpened(grpcAddress, time.Millisecond*20), nil
	}))

	randstr := md5.Sum([]byte(uuid.New().String()))
	testFileName := fmt.Sprintf("testFile_%s.txt", hex.EncodeToString(randstr[:]))
	conf.KusciaClient.KusciaV1alpha1().DomainDatas(datameshTestNamespace).Create(ctx, &v1alpha1.DomainData{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-data",
		},
		Spec: v1alpha1.DomainDataSpec{
			RelativeURI: testFileName,
			Name:        "test-data",
			Type:        "RAW",
			DataSource:  common.DefaultDataProxyDataSourceID,
			Author:      datameshTestNamespace,
		},
	}, v1.CreateOptions{})

	dialOpts := network.BuildGrpcOptions(nil)
	conn, err := grpc.Dial(grpcAddress, dialOpts...)
	assert.NoError(t, err)
	defer conn.Close()

	flightClient := flight.NewClientFromConn(conn, nil)

	toc, c1 := context.WithTimeout(ctx, time.Millisecond*1000)
	defer c1()

	// [CommandDomainDataQuery] try to read file from dataproxy
	fd, err := utils.DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: "test-data",
		ContentType:  datamesh.ContentType_RAW,
	})
	assert.NoError(t, err)
	finfo, err := flightClient.GetFlightInfo(toc, fd)
	assert.NoError(t, err)
	assert.NotNil(t, finfo)
	assert.NotEqual(t, 0, len(finfo.Endpoint))
	assert.NotNil(t, finfo.Endpoint[0])
	assert.NotNil(t, finfo.Endpoint[0].Ticket)
	assert.NotEqual(t, 0, len(finfo.Endpoint[0].Location))
	assert.NotNil(t, finfo.Endpoint[0].Location[0])
	assert.Equal(t, utils.BuiltinFlightServerEndpointURI, finfo.Endpoint[0].Location[0].Uri)

	nlog.Infof("tick %s", string(finfo.Endpoint[0].Ticket.GetTicket()))

	// create datasource test file
	localfsDir := path.Join(conf.RootDir, common.DefaultDomainDataSourceLocalFSPath)
	assert.NoError(t, paths.EnsureDirectory(localfsDir, true))

	fp, err := os.Create(path.Join(localfsDir, testFileName))
	assert.NoError(t, err)
	defer fp.Close()
	_, err = fp.WriteString("hello world!")
	assert.NoError(t, err)

	defer os.Remove(path.Join(localfsDir, testFileName))

	// read file from dataproxy
	reader, err := flightClient.DoGet(toc, finfo.Endpoint[0].Ticket)
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	r, err := flight.NewRecordReader(reader, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))
	assert.NoError(t, err)
	assert.True(t, r.Next())
	data := r.Record()
	assert.NotNil(t, data)
	assert.Equal(t, int64(1), data.NumCols())
	assert.NotNil(t, arrow.IsBaseBinary(data.Columns()[0].DataType().ID()))

	b, ok := data.Columns()[0].(*array.Binary)
	assert.True(t, ok)
	assert.NotNil(t, b)

	assert.Equal(t, 1, b.Len())
	assert.Equal(t, "hello world!", string(b.Value(0)))

	// stop all ports
	cancel()
}
