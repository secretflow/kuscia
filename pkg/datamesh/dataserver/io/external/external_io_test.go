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

package external

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
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

func registLocalFileDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dsID string) {
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Localfs: &datamesh.LocalDataSourceInfo{
			Path: "/tmp/var",
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
			Type: "localfs",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func TestNewIOServer_Failed(t *testing.T) {
	conf := config.DataProxyConfig{
		Endpoint:        "127.0.0.1:8080",
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}
	ioServer := NewIOServer(&conf)
	assert.NotNil(t, ioServer, "TestNewIOServer_Failed")
}

func TestGetFlightInfo(t *testing.T) {
	server := flight.NewServerWithMiddleware(nil)
	server.Init("localhost:0")
	srv := &MockFlightServer{}
	server.RegisterFlightService(srv)

	go server.Serve()
	defer server.Shutdown()

	conf := config.DataProxyConfig{
		Endpoint:        server.Addr().String(),
		ClientTLSConfig: nil,
		DataSourceTypes: []string{"oss"},
		Mode:            "",
	}
	ioServer := NewIOServer(&conf)
	assert.NotNil(t, ioServer)

	confi := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(confi)
	datasourceService := service.NewDomainDataSourceService(confi, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)
	registLocalFileDomainDataSource(t, confi, common.DefaultDataSourceID)
	domainDataID := registDomainData(t, confi, common.DefaultDataSourceID, "filename")
	var msg protoreflect.ProtoMessage

	msg = &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	reqCtx, err := utils.NewDataMeshRequestContext(domainDataService, datasourceService, msg)
	assert.Nil(t, err)

	fl, err := ioServer.GetFlightInfo(context.Background(), reqCtx)

	assert.Nil(t, err)
	assert.NotNil(t, fl)
	err = ioServer.DoGet(nil, nil)
	assert.NotNil(t, err)
	err = ioServer.DoPut(nil)
	assert.NotNil(t, err)
}
