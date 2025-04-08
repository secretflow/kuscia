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

package mods

import (
	"context"
	"testing"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"github.com/stretchr/testify/assert"
	"github.com/xhd2015/xgo/runtime/mock"
)

func TestCRDModSuccess(t *testing.T) {
	crdItems := []*CRDItem{
		{
			source:      "alice",
			destination: "bob",
			typ:         common.CRDDomainRoute,
		},
	}
	reporter := utils.NewReporter("")
	mod := NewCRDMod(crdItems, reporter, nil)

	crdMod := mod.(*CRDMod)
	mock.Patch(crdMod.QueryDomainRoute, func(ctx context.Context, req *kusciaapi.QueryDomainRouteRequest) (*kusciaapi.QueryDomainRouteResponse, error) {
		resp := &kusciaapi.QueryDomainRouteResponse{
			Status: &v1alpha1.Status{
				Code: 0,
			},
			Data: &kusciaapi.QueryDomainRouteResponseData{
				Status: &kusciaapi.RouteStatus{
					Status: constants.RouteSucceeded,
				},
			},
		}
		return resp, nil
	})

	mock.Patch(crdMod.CheckConnection, func(domainroute *kusciaapi.QueryDomainRouteResponseData, item *CRDItem) error {
		return nil
	})

	err := mod.Run(context.Background())
	assert.Nil(t, err)
}

func TestCRDModFail(t *testing.T) {
	crdItems := []*CRDItem{
		{
			source:      "alice",
			destination: "bob",
			typ:         common.CRDDomainRoute,
		},
	}
	reporter := utils.NewReporter("")
	mod := NewCRDMod(crdItems, reporter, nil)
	crdMod := mod.(*CRDMod)
	mock.Patch(crdMod.QueryDomainRoute, func(ctx context.Context, req *kusciaapi.QueryDomainRouteRequest) (*kusciaapi.QueryDomainRouteResponse, error) {
		resp := &kusciaapi.QueryDomainRouteResponse{
			Status: &v1alpha1.Status{
				Code: 20043,
			},
		}
		return resp, nil
	})

	err := mod.Run(context.Background())
	assert.NotNil(t, err)
}
