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
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

func TestNewQueryJobStatusAllHandler(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)

	tests := []struct {
		name       string
		wantNotNil bool
	}{
		{
			name:       "new query job status handler",
			wantNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantNotNil, NewQueryJobStatusAllHandler(rm) != nil)
		})
	}
}

func Test_queryJobStatusAllHandler_GetType(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewQueryJobStatusAllHandler(rm)

	tests := []struct {
		name         string
		wantReqType  reflect.Type
		wantRespType reflect.Type
	}{
		{
			name:         "get req and resp type",
			wantReqType:  reflect.TypeOf(QueryJobStatusAllRequest{}),
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

func Test_queryJobStatusAllHandler_Handle(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	kj := makeKusciaTask("task-1", map[string]string{common.JobIDAnnotationKey: "job-1"}, nil, "")
	kjInformer.Informer().GetStore().Add(kj)

	rm := &ResourcesManager{
		KjLister:    kjInformer.Lister(),
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	rm.InsertJob("job-1")

	h := NewQueryJobStatusAllHandler(rm)

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
			name: "kuscia job doesn't exist",
			args: args{
				ctx: nil,
				request: &QueryJobStatusAllRequest{
					JobID: "job-2",
				},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "kuscia job exist",
			args: args{
				ctx: nil,
				request: &QueryJobStatusAllRequest{
					JobID: "job-1",
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

func Test_queryJobStatusAllHandler_Validate(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewQueryJobStatusAllHandler(rm)

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
				ctx:     nil,
				request: &QueryJobStatusAllRequest{},
				errs:    &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &QueryJobStatusAllRequest{
					JobID: "job-1",
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

func Test_queryJobStatusAllHandler_buildResp(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewQueryJobStatusAllHandler(rm)

	type args struct {
		resp    *interconn.CommonResponse
		content map[string]interface{}
	}

	tests := []struct {
		name        string
		args        args
		wantContent map[string]interface{}
	}{
		{
			name: `set content to {"task-1": "Running"}`,
			args: args{
				resp: &interconn.CommonResponse{},
				content: map[string]interface{}{
					"task-1": "Running",
				},
			},
			wantContent: map[string]interface{}{
				"task-1": "Running",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hh := h.(*queryJobStatusAllHandler)
			hh.buildResp(tt.args.resp, tt.args.content)
			data := tt.args.resp.Data.AsMap()
			assert.Equal(t, tt.wantContent, data["status"])
		})
	}
}
