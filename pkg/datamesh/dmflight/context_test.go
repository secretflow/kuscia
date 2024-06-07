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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
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
			Name: dsId,
			Type: "localfs",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func TestNewDataMeshRequestContext(t *testing.T) {
	t.Parallel()
	ctx, err := NewDataMeshRequestContext(nil, nil, nil)
	assert.Error(t, err)
	assert.Nil(t, ctx)

	conf := initContextTestEnv(t)

	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// query
	ctx, err = NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "test-data",
	})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Nil(t, ctx.io)
	assert.Nil(t, ctx.update)
	assert.NotNil(t, ctx.query)
	assert.Equal(t, "", ctx.dsType)

	// update
	// query
	ctx, err = NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataUpdate{
		DomaindataId: "test-data",
	})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Nil(t, ctx.io)
	assert.NotNil(t, ctx.update)
	assert.Nil(t, ctx.query)
	assert.Equal(t, "", ctx.dsType)
}

func TestGetDataSourceType(t *testing.T) {
	t.Parallel()
	conf := initContextTestEnv(t)
	domainDataService := service.NewDomainDataService(conf)
	datasourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, datasourceService)

	// domain-data not exists
	ctx, err := NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: "test-data",
	})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	dsType, err := ctx.GetDataSourceType()
	assert.Error(t, err)
	assert.Equal(t, "", dsType)

	// init ok
	domainDataID := registDomainData(t, conf, common.DefaultDataSourceID, "localtest.txt")
	ctx, err = NewDataMeshRequestContext(domainDataService, datasourceService, &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	})

	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	// datasource not exists
	dsType, err = ctx.GetDataSourceType()
	assert.Error(t, err)
	assert.Equal(t, "", dsType)

	registLocalFileDomainDataSource(t, conf, common.DefaultDataSourceID)
	dsType, err = ctx.GetDataSourceType()
	assert.NoError(t, err)
	assert.Equal(t, "localfs", dsType)
	assert.Equal(t, "localfs", ctx.dsType)
}

func TestGetTransferType(t *testing.T) {
	t.Parallel()
	// query
	ctx, err := NewDataMeshRequestContext(nil, nil, &datamesh.CommandDomainDataQuery{
		DomaindataId: "test-data",
		ContentType:  datamesh.ContentType_RAW,
	})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	assert.Equal(t, "test-data", ctx.getDomainDataID())
	assert.Equal(t, datamesh.ContentType_RAW, ctx.GetTransferContentType())

	// update
	ctx, err = NewDataMeshRequestContext(nil, nil, &datamesh.CommandDomainDataUpdate{
		DomaindataId: "test-data",
		ContentType:  datamesh.ContentType_RAW,
	})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	assert.Equal(t, "test-data", ctx.getDomainDataID())
	assert.Equal(t, datamesh.ContentType_RAW, ctx.GetTransferContentType())
}
