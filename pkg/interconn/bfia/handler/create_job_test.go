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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/adapter"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

func makeInterConnConfig() *adapter.InterConnJobInfo {
	interConnJob := `{"dag":{"version":"2.0.0","components":[{"code":"secretflow","name":"intersect_rsa_1","module_name":"intersect_rsa","version":"latest","input":[{"type":"dataset","key":"intersect_rsa_1.data0"}],"output":[{"type":"training_set","key":"data0"},{"type":"test_set","key":"data1"},{"type":"model","key":"data2"}]},{"code":"secretflow","name":"hetero_logistic_regression_1","module_name":"hetero_logistic_regression","version":"v1.0.0","input":[{"type":"training_set","key":"data0"}],"output":[{"type":"training_set","key":"data1"},{"type":"test_set","key":"data1"},{"type":"model","key":"data2"}]}]},"config":{"role":{"arbiter":["JG0110017800000000"],"host":["JG0100001100000000"],"guest":["JG0110017800000000"]},"initiator":{"role":"guest","node_id":"JG0110017800000000"},"job_params":{"host":{"0":{}},"arbiter":{"0":{}},"guest":{"0":{}}},"task_params":{"host":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_host","namespace":"testspace"}}},"arbiter":{"0":{"intersect_rsa_1":{}}},"guest":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_guest","namespace":"testspace"}}},"common":{"hetero_logistic_regression_1":{"C":0.01,"batch_size":-1,"level":"4","penalty":"L2"},"intersect_rsa_1":{"intersect_method":"rsa","only_output_key":false,"rsa_params":{"final_hash_method":"sha256","hash_method":"sha256","key_length":2048},"sync_intersect_ids":true}}},"version":"2.0.0"},"job_id":"secretflow","flow_id":"flow-1"}`
	jobInfo := &adapter.InterConnJobInfo{}
	json.Unmarshal([]byte(interConnJob), jobInfo)
	return jobInfo
}

func TestNewCreateJobHandler(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)

	tests := []struct {
		name       string
		wantNotNil bool
	}{
		{
			name:       "new create job handler",
			wantNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantNotNil, NewCreateJobHandler(rm) != nil)
		})
	}
}

func Test_createJobHandler_GetType(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewCreateJobHandler(rm)

	tests := []struct {
		name         string
		wantReqType  reflect.Type
		wantRespType reflect.Type
	}{
		{
			name:         "get req and resp type",
			wantReqType:  reflect.TypeOf(interconn.CreateJobRequest{}),
			wantRespType: reflect.TypeOf(interconn.CommonResponse{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReqType, gotRespType := h.GetType()
			assert.Equal(t, tt.wantReqType, gotReqType)
			assert.Equal(t, tt.wantRespType, gotRespType)
		})
	}
}

func Test_createJobHandler_Handle(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	icJobInfo := makeInterConnConfig()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewCreateJobHandler(rm)

	type args struct {
		ctx     *api.BizContext
		request api.ProtoRequest
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "req is invalid",
			args: args{
				ctx:     nil,
				request: &interconn.CreateJobRequest{},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &interconn.CreateJobRequest{
					Dag:    icJobInfo.DAG,
					Config: icJobInfo.Config,
					FlowId: icJobInfo.FlowID,
					JobId:  icJobInfo.JodID,
				},
			},
			wantCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := h.Handle(tt.args.ctx, tt.args.request)
			got := resp.(*interconn.CommonResponse)
			assert.Equal(t, tt.wantCode, got.Code)
		})
	}
}

func Test_createJobHandler_Validate(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	icJobInfo := makeInterConnConfig()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewCreateJobHandler(rm)

	type args struct {
		ctx     *api.BizContext
		request api.ProtoRequest
		errs    *errorcode.Errs
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "req type is invalid",
			args: args{
				ctx:     nil,
				request: &interconn.StartJobRequest{},
				errs:    &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req job id is empty",
			args: args{
				ctx: nil,
				request: &interconn.CreateJobRequest{
					Dag:    icJobInfo.DAG,
					Config: icJobInfo.Config,
				},
				errs: &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req config is empty",
			args: args{
				ctx: nil,
				request: &interconn.CreateJobRequest{
					JobId: icJobInfo.JodID,
					Dag:   icJobInfo.DAG,
				},
				errs: &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req dag is empty",
			args: args{
				ctx: nil,
				request: &interconn.CreateJobRequest{
					JobId:  icJobInfo.JodID,
					Config: icJobInfo.Config,
				},
				errs: &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &interconn.CreateJobRequest{
					JobId:  icJobInfo.JodID,
					Config: icJobInfo.Config,
					Dag:    icJobInfo.DAG,
				},
				errs: &errorcode.Errs{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.Validate(tt.args.ctx, tt.args.request, tt.args.errs)
			assert.Equal(t, tt.wantErr, len(*tt.args.errs) != 0)
		})
	}
}
