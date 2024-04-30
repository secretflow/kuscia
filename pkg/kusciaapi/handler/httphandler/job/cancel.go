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

package job

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type cancelJobHandler struct {
	jobService service.IJobService
}

func NewCancelJobHandler(jobService service.IJobService) api.ProtoHandler {
	return &cancelJobHandler{
		jobService: jobService,
	}
}

func (q cancelJobHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
}

func (q cancelJobHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req, _ := request.(*kusciaapi.CancelJobRequest)
	return q.jobService.CancelJob(context.Context, req)
}

func (q cancelJobHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(kusciaapi.CancelJobRequest{}), reflect.TypeOf(kusciaapi.CancelJobResponse{})
}
