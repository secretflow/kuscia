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

package handler

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/reporter/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/reporter"
)

type reportProgressHandler struct {
	reportService service.IReportService
}

func NewReportProgressHandler(reportService service.IReportService) api.ProtoHandler {
	return &reportProgressHandler{
		reportService: reportService,
	}
}

func (r reportProgressHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (r reportProgressHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	reportRequest, _ := request.(*reporter.ReportProgressRequest)
	return r.reportService.ReportProgress(context.Context, reportRequest, context.Query("task_id"))
}

func (r reportProgressHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(reporter.ReportProgressRequest{}), reflect.TypeOf(reporter.CommonReportResponse{})
}

type healthHandler struct {
	healthService service.IHealthService
}

func NewHealthHandler(healthService service.IHealthService) api.ProtoHandler {
	return &healthHandler{
		healthService: healthService,
	}
}

func (h healthHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (h healthHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	healthRequest, _ := request.(*reporter.HealthRequest)
	return h.healthService.HealthCheck(context.Context, healthRequest)
}

func (h healthHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(reporter.ReportProgressRequest{}), reflect.TypeOf(reporter.CommonReportResponse{})
}
