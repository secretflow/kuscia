// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func TestCreateDomainDataGrant(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	dir, err := os.MkdirTemp("", "TestCreateDomainDataGrant")
	assert.NoError(t, err)
	cafile := filepath.Join(dir, "ca.key")

	keyOut, err := os.OpenFile(cafile, os.O_CREATE|os.O_RDWR, 0644)
	assert.NoError(t, err)
	defer keyOut.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	assert.NoError(t, err)
	conf := &config.DataMeshConfig{
		KusciaClient:  kusciafake.NewSimpleClientset(),
		KubeNamespace: "DomainDataUnitTestNamespace",
		DomainKeyFile: cafile,
	}

	domainDataService := NewDomainDataService(conf)
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	assert.Equal(t, int32(0), res.Status.Code)
	domainDataGrantService := NewDomainDataGrantService(conf)
	tm := metav1.Now().Add(30 * time.Second)
	data := &datamesh.DomainDataGrantData{
		DomaindatagrantId: "",
		Author:            conf.KubeNamespace,
		DomaindataId:      res.Data.DomaindataId,
		GrantDomain:       "bob",
		Limit: &datamesh.GrantLimit{
			ExpirationTime: tm.UnixNano(),
			UseCount:       3,
			FlowId:         "bbbb",
			Componets:      []string{"mpc", "psi"},
			Initiator:      conf.KubeNamespace,
			InputConfig:    "xxxxx",
		},
		Description: map[string]string{
			"test": "xxx",
		},
		Signature: "ssssss",
	}
	resp := domainDataGrantService.CreateDomainDataGrant(context.Background(), &datamesh.CreateDomainDataGrantRequest{
		DomaindataId: data.DomaindataId,
		GrantDomain:  data.GrantDomain,
		Limit:        data.Limit,
		Description:  data.Description,
	})

	assert.Equal(t, int32(0), resp.Status.Code)
	time.Sleep(time.Millisecond * 100)
	dg, err := conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(conf.KubeNamespace).Get(context.Background(), resp.Data.DomaindatagrantId, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, dg.Spec.Author, conf.KubeNamespace)
	assert.True(t, data.Signature != dg.Spec.Signature) // signature is auto generate
	createSign := dg.Spec.Signature

	queryResp := domainDataGrantService.QueryDomainDataGrant(context.Background(), &datamesh.QueryDomainDataGrantRequest{
		DomaindatagrantId: resp.Data.DomaindatagrantId,
	})
	assert.Equal(t, queryResp.Data.Author, conf.KubeNamespace)

	domainDataGrantService.UpdateDomainDataGrant(context.Background(), &datamesh.UpdateDomainDataGrantRequest{
		DomaindatagrantId: resp.Data.DomaindatagrantId,
		DomaindataId:      data.DomaindataId,
		GrantDomain:       "carol",
		Limit:             data.Limit,
		Description:       data.Description,
	})

	time.Sleep(time.Millisecond * 100)
	dg, err = conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(conf.KubeNamespace).Get(context.Background(), resp.Data.DomaindatagrantId, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "carol", dg.Spec.GrantDomain)
	assert.True(t, createSign != dg.Spec.Signature) // signature is auto generate

	domainDataGrantService.DeleteDomainDataGrant(context.Background(), &datamesh.DeleteDomainDataGrantRequest{
		DomaindatagrantId: resp.Data.DomaindatagrantId,
	})
	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(conf.KubeNamespace).Get(context.Background(), resp.Data.DomaindatagrantId, metav1.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))
}
