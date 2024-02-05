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

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

func TestNewStopTaskHandler(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)

	tests := []struct {
		name       string
		wantNotNil bool
	}{
		{
			name:       "new stop task handler",
			wantNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantNotNil, NewStopTaskHandler(rm) != nil)
		})
	}
}

func Test_stopTaskHandler_GetType(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewStopTaskHandler(rm)

	tests := []struct {
		name         string
		wantReqType  reflect.Type
		wantRespType reflect.Type
	}{
		{
			name:         "get req and resp type",
			wantReqType:  reflect.TypeOf(interconn.StopTaskRequest{}),
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

func Test_stopTaskHandler_Handle(t *testing.T) {
	kt1 := makeKusciaTask("task-1", nil, nil, kusciaapisv1alpha1.TaskSucceeded)
	kt2 := makeKusciaTask("task-2", nil, nil, kusciaapisv1alpha1.TaskRunning)

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kt1, kt2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	kt3 := makeKusciaTask("task-3", nil, nil, kusciaapisv1alpha1.TaskRunning)
	ktInformer.Informer().GetStore().Add(kt1)
	ktInformer.Informer().GetStore().Add(kt2)
	ktInformer.Informer().GetStore().Add(kt3)

	rm := &ResourcesManager{
		KusciaClient: kusciaFakeClient,
		KtLister:     ktInformer.Lister(),
		jobTaskInfo:  make(map[string]map[string]struct{}),
		taskJobInfo:  make(map[string]string),
	}
	h := NewStopTaskHandler(rm)

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
				request: &interconn.StopTaskRequest{
					TaskId: "task-11",
				},
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "req task phase already succeeded",
			args: args{
				ctx: nil,
				request: &interconn.StopTaskRequest{
					TaskId: "task-1",
				},
			},
			wantCode: http.StatusOK,
		},
		{
			name: "stop task failed",
			args: args{
				ctx: nil,
				request: &interconn.StopTaskRequest{
					TaskId: "task-3",
				},
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "stop task succeeded",
			args: args{
				ctx: nil,
				request: &interconn.StopTaskRequest{
					TaskId: "task-2",
				},
			},
			wantCode: http.StatusOK,
		},
		{
			name: "stop task succeeded",
			args: args{
				ctx: nil,
				request: &interconn.StopTaskRequest{
					TaskId: "task-2",
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

func Test_stopTaskHandler_Validate(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	rm, _ := NewResourcesManager(ctx, kusciaFakeClient)
	h := NewStopTaskHandler(rm)

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
				ctx:     nil,
				request: &interconn.StopTaskRequest{},
				errs:    &errorcode.Errs{},
			},
			wantErr: true,
		},
		{
			name: "req is valid",
			args: args{
				ctx: nil,
				request: &interconn.StopTaskRequest{
					TaskId: "task-1",
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
