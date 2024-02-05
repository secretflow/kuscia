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

//nolint:dulp
package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func TestCreateDomainRouteWithAllDomainNotExists(t *testing.T) {
	res := createDomainRoute()
	t.Logf("CreateDomainRoute res : %+v\n", res)
	assert.NotNil(t, res)
	assert.Equal(t, int32(errorcode.ErrDomainNotExists), res.Status.Code)
}

func TestCreateDomainRouteWithSourceDomainNotExists(t *testing.T) {

	domainSourceDestination := CreateDomain(kusciaAPIDR.destination)
	t.Logf("CreateDomainRoute destination res : %+v\n", domainSourceDestination)
	assert.NotNil(t, domainSourceDestination)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainSourceDestination.Status.Code)

	res := createDomainRoute()
	t.Log(fmt.Sprintf("CreateDomainRoute res : %+v\n", res))
	assert.NotNil(t, res)
	assert.Equal(t, int32(errorcode.ErrDomainNotExists), res.Status.Code)
}

func TestCreateDomainRoute(t *testing.T) {

	domainSourceRes := CreateDomain(kusciaAPIDR.source)
	t.Logf("CreateDomainRoute source res : %+v\n", domainSourceRes)
	assert.NotNil(t, domainSourceRes)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainSourceRes.Status.Code)

	res := createDomainRoute()
	t.Log(fmt.Sprintf("CreateDomainRoute res : %+v\n", res))
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)
}

func TestQueryRoute(t *testing.T) {
	res := kusciaAPIDR.QueryDomainRoute(context.Background(), &kusciaapi.QueryDomainRouteRequest{
		Source:      kusciaAPIDR.source,
		Destination: kusciaAPIDR.destination,
	})
	t.Logf("QueryRoute: res : %+v\n", res)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Data)
	assert.Equal(t, res.Data.Source, kusciaAPIDR.source)
	assert.Equal(t, res.Data.Destination, kusciaAPIDR.destination)
}

func TestBatchQueryRoutes(t *testing.T) {
	res := kusciaAPIDR.BatchQueryDomainRouteStatus(context.Background(), &kusciaapi.BatchQueryDomainRouteStatusRequest{
		RouteKeys: []*kusciaapi.DomainRouteKey{
			{
				Source:      kusciaAPIDR.source,
				Destination: kusciaAPIDR.destination,
			},
		},
	})
	assert.Equal(t, len(res.Data.Routes), 1)
}

func TestDeleteRoute(t *testing.T) {
	deleteRes := kusciaAPIDR.DeleteDomainRoute(context.Background(), &kusciaapi.DeleteDomainRouteRequest{
		Source:      kusciaAPIDR.source,
		Destination: kusciaAPIDR.destination,
	})
	assert.NotNil(t, deleteRes)
	queryRes := kusciaAPIDR.QueryDomainRoute(context.Background(), &kusciaapi.QueryDomainRouteRequest{
		Source:      kusciaAPIDR.source,
		Destination: kusciaAPIDR.destination,
	})
	assert.Equal(t, queryRes.Status.Code, int32(errorcode.ErrDomainRouteNotExists))
}

func TestConvertDomainRouteProtocol(t *testing.T) {
	p, isTLS, err := convert2DomainRouteProtocol("http")
	assert.False(t, isTLS)
	assert.Nil(t, err)
	assert.Equal(t, p, v1alpha1.DomainRouteProtocolHTTP)

	p, isTLS, err = convert2DomainRouteProtocol("https")
	assert.True(t, isTLS)
	assert.Nil(t, err)
	assert.Equal(t, p, v1alpha1.DomainRouteProtocolHTTP)

	p, isTLS, err = convert2DomainRouteProtocol("grpc")
	assert.False(t, isTLS)
	assert.Nil(t, err)
	assert.Equal(t, p, v1alpha1.DomainRouteProtocolGRPC)

	_, _, err = convert2DomainRouteProtocol("xxx")
	assert.NotNil(t, err)
}

func createDomainRoute() *kusciaapi.CreateDomainRouteResponse {

	return kusciaAPIDR.CreateDomainRoute(context.Background(), &kusciaapi.CreateDomainRouteRequest{
		Source:             kusciaAPIDR.source,
		Destination:        kusciaAPIDR.destination,
		AuthenticationType: string(v1alpha1.DomainAuthenticationToken),
		Endpoint: &kusciaapi.RouteEndpoint{
			Host: "localhost",
			Ports: []*kusciaapi.EndpointPort{
				{
					Name:     "GRPC",
					Port:     8080,
					Protocol: "GRPC",
				},
			},
		},
		TokenConfig: &kusciaapi.TokenConfig{
			DestinationPublicKey: "dest pk",
			SourcePublicKey:      "source pk",
			TokenGenMethod:       "RSA",
		},
	})
}
