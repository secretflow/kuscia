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
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	iccommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

func TestNewPollTaskStatusHandler(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)

	tests := []struct {
		name       string
		wantNotNil bool
	}{
		{
			name:       "new poll task status handler",
			wantNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantNotNil, NewPollTaskStatusHandler(rm) != nil)
		})
	}
}

func Test_pollTaskStatusHandler_GetType(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewPollTaskStatusHandler(rm)

	tests := []struct {
		name         string
		wantReqType  reflect.Type
		wantRespType reflect.Type
	}{
		{
			name:         "get req and resp type",
			wantReqType:  reflect.TypeOf(interconn.PollTaskStatusRequest{}),
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

func Test_pollTaskStatusHandler_Handle(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	kt := makeKusciaTask("task-1", map[string]string{common.LabelJobID: "job-1"}, "")
	ktInformer.Informer().GetStore().Add(kt)

	rm := &ResourcesManager{
		KtLister:    ktInformer.Lister(),
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	rm.InsertJob("job-1")
	rm.InsertTask("job-1", "task-1")

	h := NewPollTaskStatusHandler(rm)

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
			name: "req task id doesn't exist",
			args: args{
				ctx: nil,
				request: &interconn.PollTaskStatusRequest{
					TaskId: "task-2",
					Role:   "host",
				},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &interconn.PollTaskStatusRequest{
					TaskId: "task-1",
					Role:   "host",
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

func Test_pollTaskStatusHandler_Validate(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewPollTaskStatusHandler(rm)

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
			name: "req task id is empty",
			args: args{
				ctx: nil,
				request: &interconn.PollTaskStatusRequest{
					Role: "host",
				},
				errs: &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req role is empty",
			args: args{
				ctx: nil,
				request: &interconn.PollTaskStatusRequest{
					TaskId: "task-1",
				},
				errs: &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &interconn.PollTaskStatusRequest{
					TaskId: "task-1",
					Role:   "host",
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

func Test_pollTaskStatusHandler_buildResp(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewPollTaskStatusHandler(rm)

	type args struct {
		resp  *interconn.CommonResponse
		phase v1alpha1.KusciaTaskPhase
	}

	tests := []struct {
		name      string
		args      args
		wantPhase string
	}{
		{
			name: "set phase to Running",
			args: args{
				resp:  &interconn.CommonResponse{},
				phase: v1alpha1.TaskRunning,
			},
			wantPhase: iccommon.InterConnRunning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hh := h.(*pollTaskStatusHandler)
			hh.buildResp(tt.args.resp, tt.args.phase)
			data := tt.args.resp.Data.AsMap()
			assert.Equal(t, tt.wantPhase, data["status"])
		})
	}
}
