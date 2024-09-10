// Copyright 2023 Ant Group Co., Ltd.
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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/driver"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func TestCreateDefaultDomainDataSource(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	domainDataService := makeDomainDataSourceService(t, conf)
	res := domainDataService.CreateDefaultDomainDataSource(context.Background())
	assert.Nil(t, res)
}

func TestQueryDomainDataSource(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKey:     key,
	}
	domainDataService := makeDomainDataSourceService(t, conf)
	err = domainDataService.CreateDefaultDomainDataSource(context.Background())
	assert.Nil(t, err)
	queryRes := domainDataService.QueryDomainDataSource(context.Background(), &datamesh.QueryDomainDataSourceRequest{
		DatasourceId: common.DefaultDataSourceID,
	})
	assert.NotNil(t, queryRes)
	assert.Equal(t, queryRes.Data.Type, common.DomainDataSourceTypeLocalFS)
	assert.Equal(t, queryRes.Data.Info.Localfs.Path, common.DefaultDomainDataSourceLocalFSPath)
}

func makeDomainDataSourceService(t *testing.T, conf *config.DataMeshConfig) IDomainDataSourceService {
	return NewDomainDataSourceService(conf, makeConfigService(t))
}

func makeConfigService(t *testing.T) cmservice.IConfigService {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	assert.Nil(t, err)

	encValue, err := tls.EncryptOAEP(&privateKey.PublicKey, []byte("test-value"))
	assert.Nil(t, err)

	configmap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "alice",
			Name:      "domain-config",
		},
		Data: map[string]string{
			"test-key": encValue,
		},
	}
	kubeClient := kubefake.NewSimpleClientset(&configmap)
	configService, err := cmservice.NewConfigService(context.Background(), &cmservice.ConfigServiceConfig{
		DomainID:     "alice",
		DomainKey:    privateKey,
		Driver:       driver.CRDDriverType,
		DisableCache: true,
		KubeClient:   kubeClient,
	})
	assert.Nil(t, err)
	return configService
}
