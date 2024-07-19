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

//nolint:dupl
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func TestDomainDataGrant(t *testing.T) {

	domainRes := CreateDomain(kusciaAPIDG.data.DomainId)
	t.Logf("CreateDomain res : %+v\n", domainRes)
	assert.NotNil(t, domainRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainRes.Status.Code)

	kusciaAPIDG.conf.KusciaClient.KusciaV1alpha1().DomainDatas(kusciaAPIDG.data.Author).Create(context.Background(), &v1alpha1.DomainData{
		ObjectMeta: v1.ObjectMeta{
			Name:      kusciaAPIDG.data.DomaindataId,
			Namespace: kusciaAPIDG.data.Author,
		},
	}, v1.CreateOptions{})
	createRes := kusciaAPIDG.CreateDomainDataGrant(context.Background(), &kusciaapi.CreateDomainDataGrantRequest{
		DomaindatagrantId: kusciaAPIDG.data.DomaindatagrantId,
		DomaindataId:      kusciaAPIDG.data.DomaindataId,
		GrantDomain:       kusciaAPIDG.data.GrantDomain,
		Limit:             kusciaAPIDG.data.Limit,
		Description:       kusciaAPIDG.data.Description,
		Signature:         kusciaAPIDG.data.Signature,
		DomainId:          kusciaAPIDG.data.DomainId,
	})
	assert.NotNil(t, createRes)
	t.Log(createRes)
	queryRes := kusciaAPIDG.QueryDomainDataGrant(context.Background(), &kusciaapi.QueryDomainDataGrantRequest{
		DomainId:          kusciaAPIDG.data.DomainId,
		DomaindatagrantId: createRes.Data.DomaindatagrantId,
	})
	assert.Equal(t, int32(0), queryRes.Status.Code)

	batchQueryRes := kusciaAPIDG.BatchQueryDomainDataGrant(context.Background(), &kusciaapi.BatchQueryDomainDataGrantRequest{
		Data: []*kusciaapi.QueryDomainDataGrantRequestData{
			{
				DomainId:          kusciaAPIDG.data.DomainId,
				DomaindatagrantId: createRes.Data.DomaindatagrantId,
			},
		},
	})
	assert.Equal(t, batchQueryRes.Status.Code, int32(0))

	updateRes := kusciaAPIDG.UpdateDomainDataGrant(context.Background(), &kusciaapi.UpdateDomainDataGrantRequest{
		DomaindatagrantId: createRes.Data.DomaindatagrantId,
		DomaindataId:      kusciaAPIDG.data.DomaindataId,
		GrantDomain:       "carol",
		Limit:             kusciaAPIDG.data.Limit,
		Description:       kusciaAPIDG.data.Description,
		Signature:         kusciaAPIDG.data.Signature,
		DomainId:          kusciaAPIDG.data.DomainId,
	})
	t.Log(updateRes)
	assert.Equal(t, int32(0), updateRes.Status.Code)

	listRes := kusciaAPIDG.ListDomainDataGrant(context.Background(), &kusciaapi.ListDomainDataGrantRequest{
		Data: &kusciaapi.ListDomainDataGrantRequestData{
			DomainId: kusciaAPIDG.data.DomainId,
		},
	})
	assert.Equal(t, listRes.Status.Code, int32(0))

	deleteRes := kusciaAPIDG.DeleteDomainDataGrant(context.Background(), &kusciaapi.DeleteDomainDataGrantRequest{
		DomainId:          kusciaAPIDG.data.DomainId,
		DomaindatagrantId: createRes.Data.DomaindatagrantId,
	})
	assert.Equal(t, deleteRes.Status.Code, int32(0))
}
