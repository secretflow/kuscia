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
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"github.com/stretchr/testify/assert"
)

func TestDomainRouteMod(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		PeerEndpoint: "",
		Manual:       false,
		NetworkParam: *netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewDomainRouteMod(reporter, nil, conf)
	drMod := mod.(*DomainRouteMod)

	patch1 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "QueryDomainData", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.QueryDomainDataRequest) (*kusciaapi.QueryDomainDataResponse, error) {
		resp := &kusciaapi.QueryDomainDataResponse{
			Status: &v1alpha1.Status{
				Code: 0,
			},
			Data: &kusciaapi.DomainData{
				Attributes: map[string]string{
					"BANDWIDTH": `{"name":"BANDWIDTH","detected_value":"7413.75Mbits/sec","threshold":"10Mbits/sec","result":"[PASS]"}`,
				},
			},
		}
		return resp, nil
	})
	defer patch1.Reset()

	patch2 := gomonkey.ApplyMethod(reflect.TypeOf(drMod.crdMod), "Run", func(_ *CRDMod, ctx context.Context) error {
		return nil
	})
	defer patch2.Reset()

	patch3 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "CreateJob", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.CreateJobRequest) error {
		return nil
	})
	defer patch3.Reset()

	patch4 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobStart", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch4.Reset()

	patch5 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobDone", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch5.Reset()

	err := mod.Run(context.Background())
	assert.Nil(t, err)
}

func TestDomainRouteManualMod(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		PeerEndpoint: "",
		Manual:       true,
		NetworkParam: *netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewDomainRouteMod(reporter, nil, conf)
	drMod := mod.(*DomainRouteMod)

	patch1 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "QueryDomainData", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.QueryDomainDataRequest) (*kusciaapi.QueryDomainDataResponse, error) {
		resp := &kusciaapi.QueryDomainDataResponse{
			Status: &v1alpha1.Status{
				Code: 0,
			},
			Data: &kusciaapi.DomainData{
				Attributes: map[string]string{
					"BANDWIDTH": `{"name":"BANDWIDTH","detected_value":"7413.75Mbits/sec","threshold":"10Mbits/sec","result":"[PASS]"}`,
				},
			},
		}
		return resp, nil
	})
	defer patch1.Reset()

	patch2 := gomonkey.ApplyMethod(reflect.TypeOf(drMod.crdMod), "Run", func(_ *CRDMod, ctx context.Context) error {
		return nil
	})
	defer patch2.Reset()

	patch3 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "CreateJob", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.CreateJobRequest) error {
		return nil
	})
	defer patch3.Reset()

	patch4 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobStart", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch4.Reset()

	patch5 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobDone", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch5.Reset()

	err := mod.Run(context.Background())
	assert.Nil(t, err)
}

func TestDomainRouteModFail(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		PeerEndpoint: "",
		Manual:       false,
		NetworkParam: *netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewDomainRouteMod(reporter, nil, conf)
	drMod := mod.(*DomainRouteMod)

	patch1 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "QueryDomainData", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.QueryDomainDataRequest) (*kusciaapi.QueryDomainDataResponse, error) {
		resp := &kusciaapi.QueryDomainDataResponse{
			Status: &v1alpha1.Status{
				Code: 0,
			},
			Data: &kusciaapi.DomainData{
				Attributes: map[string]string{
					"BANDWIDTH": `{"name":"BANDWIDTH","detected_value":"7413.75Mbits/sec","threshold":"10Mbits/sec","result":"[PASS]"}`,
				},
			},
		}
		return resp, nil
	})
	defer patch1.Reset()

	patch2 := gomonkey.ApplyMethod(reflect.TypeOf(drMod.crdMod), "Run", func(_ *CRDMod, ctx context.Context) error {
		return nil
	})
	defer patch2.Reset()

	patch3 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "CreateJob", func(_ *DomainRouteMod, ctx context.Context, req *kusciaapi.CreateJobRequest) error {
		return fmt.Errorf("create job failed")
	})
	defer patch3.Reset()

	patch4 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobStart", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch4.Reset()

	patch5 := gomonkey.ApplyMethod(reflect.TypeOf(mod), "WaitJobDone", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch5.Reset()

	err := mod.Run(context.Background())
	assert.NotNil(t, err)
}
