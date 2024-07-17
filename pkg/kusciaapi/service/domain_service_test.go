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

//nolint:dupl
package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"github.com/secretflow/kuscia/test/util"
)

func TestCreateDomain(t *testing.T) {
	res := kusciaAPIDS.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: kusciaAPIDS.domainID,
	})
	assert.NotNil(t, res)
}

func TestCreateDomain_NameError(t *testing.T) {
	t.Parallel()
	ds := &domainService{}
	res := ds.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: "",
	})
	assert.NotNil(t, res)
	assert.Equal(t, int32(errorcode.ErrorCode_KusciaAPIErrRequestValidate), res.Status.Code)

	res = ds.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: "master",
	})
	assert.NotNil(t, res)
	assert.Equal(t, int32(errorcode.ErrorCode_KusciaAPIErrRequestValidate), res.Status.Code)
}

func TestCreateDomainWithCertError(t *testing.T) {
	res := kusciaAPIDS.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: kusciaAPIDS.domainID,
		Cert:     "cert",
	})
	assert.NotNil(t, res)
	assert.Equal(t, res.Status.Code, int32(errorcode.ErrorCode_KusciaAPIErrRequestValidate))
}

func TestCreateDomainWithCertSuccess(t *testing.T) {
	res := kusciaAPIDS.CreateDomain(context.Background(), &kusciaapi.CreateDomainRequest{
		DomainId: "test-create-domain-with-cert-success",
		Cert:     util.MakeBase64EncodeCert(t),
	})
	assert.NotNil(t, res)
	assert.Equal(t, res.Status.Code, kusciaAPISuccessStatusCode)
}

func TestQueryDomain(t *testing.T) {
	res := kusciaAPIDS.QueryDomain(context.Background(), &kusciaapi.QueryDomainRequest{
		DomainId: kusciaAPIDS.domainID,
	})
	assert.Equal(t, res.Data.DomainId, kusciaAPIDS.domainID)
}

func TestUpdateDomain(t *testing.T) {
	res := kusciaAPIDS.UpdateDomain(context.Background(), &kusciaapi.UpdateDomainRequest{
		DomainId: kusciaAPIDS.domainID,
		Cert:     "cert",
	})
	assert.NotNil(t, res)
}

func TestUpdateDomainWithCertError(t *testing.T) {
	res := kusciaAPIDS.UpdateDomain(context.Background(), &kusciaapi.UpdateDomainRequest{
		DomainId: kusciaAPIDS.domainID,
		Cert:     "cert",
	})
	assert.NotNil(t, res)
	assert.Equal(t, res.Status.Code, int32(errorcode.ErrorCode_KusciaAPIErrRequestValidate))
}

func TestUpdateDomainWithCertSuccess(t *testing.T) {
	res := kusciaAPIDS.UpdateDomain(context.Background(), &kusciaapi.UpdateDomainRequest{
		DomainId: kusciaAPIDS.domainID,
		Cert:     util.MakeBase64EncodeCert(t),
	})
	assert.NotNil(t, res)
	assert.Equal(t, res.Status.Code, kusciaAPISuccessStatusCode)
}

func TestBatchQueryDomain(t *testing.T) {
	res := kusciaAPIDS.BatchQueryDomain(context.Background(), &kusciaapi.BatchQueryDomainRequest{
		DomainIds: []string{kusciaAPIDS.domainID},
	})
	assert.Equal(t, len(res.Data.Domains), 1)
}

func TestDeleteDomain(t *testing.T) {
	deleteRes := kusciaAPIDS.DeleteDomain(context.Background(), &kusciaapi.DeleteDomainRequest{
		DomainId: kusciaAPIDS.domainID,
	})
	assert.NotNil(t, deleteRes)
	queryRes := kusciaAPIDS.QueryDomain(context.Background(), &kusciaapi.QueryDomainRequest{
		DomainId: kusciaAPIDS.domainID,
	})
	assert.Equal(t, queryRes.Status.Code, int32(errorcode.ErrorCode_KusciaAPIErrDomainNotExists))
}

func TestGetValidCert(t *testing.T) {
	t.Parallel()

	rawContent := util.MakeCertString(t)

	//var res string
	var err error

	ds := &domainService{}
	assert.NotNil(t, ds)

	_, err = ds.getValidCert(rawContent)
	assert.NoError(t, err)

	// base64 1 times
	input1 := base64.StdEncoding.EncodeToString([]byte(rawContent))
	_, err = ds.getValidCert(input1)
	assert.NoError(t, err)

	// remove begin/end
	input2 := strings.TrimSuffix(strings.TrimPrefix(rawContent, "-----BEGIN CERTIFICATE-----"), "-----END CERTIFICATE-----")
	_, err = ds.getValidCert(input2)
	assert.NoError(t, err)

	// remove all "\n"
	input3 := strings.ReplaceAll(input2, "\n", "")
	_, err = ds.getValidCert(input3)
	assert.NoError(t, err)

	// base64 2 times
	input4 := base64.StdEncoding.EncodeToString([]byte(input1))
	_, err = ds.getValidCert(input4)
	assert.NoError(t, err)

	// has "\r"
	input5 := strings.ReplaceAll(rawContent, "\n", "\r\n")
	_, err = ds.getValidCert(input5)
	assert.NoError(t, err)
}
