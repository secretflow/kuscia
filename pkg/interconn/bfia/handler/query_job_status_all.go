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
	"fmt"
	"net/http"
	"reflect"

	"google.golang.org/protobuf/types/known/structpb"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/secretflow/kuscia/pkg/common"
	bfiacommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

// QueryJobStatusAllRequest defines the request body info for querying job status.
type QueryJobStatusAllRequest struct {
	api.ProtoRequest
	JobID string `form:"job_id"`
}

// queryJobStatusAllHandler defines the handler info for querying job status.
type queryJobStatusAllHandler struct {
	*ResourcesManager
}

// NewQueryJobStatusAllHandler returns a queryJobStatusAllHandler instance.
func NewQueryJobStatusAllHandler(rm *ResourcesManager) api.ProtoHandler {
	return &queryJobStatusAllHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *queryJobStatusAllHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*QueryJobStatusAllRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("query job status request type is invalid"))
		return
	}

	if req.JobID == "" {
		errs.AppendErr(fmt.Errorf("parameter job_id can't be empty"))
	}
}

// Handle is used to handle request.
func (h *queryJobStatusAllHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*QueryJobStatusAllRequest)
	resp := &interconn.CommonResponse{
		Code: http.StatusOK,
	}

	kj, err := h.KjLister.KusciaJobs(common.KusciaCrossDomain).Get(req.JobID)
	if k8serrors.IsNotFound(err) {
		if h.IsJobExist(req.JobID) {
			h.buildResp(resp, nil)
			return resp
		}
	}

	if err != nil {
		resp.Code = http.StatusBadRequest
		resp.Msg = bfiacommon.ErrFindJobFailed
		return resp
	}

	statusContent := make(map[string]interface{})
	for taskID, taskStatus := range kj.Status.TaskStatus {
		statusContent[taskID] = bfiacommon.KusciaTaskPhaseToInterConnTaskPhase[taskStatus]
	}

	h.buildResp(resp, statusContent)
	return resp
}

// GetType is used to get request and response type.
func (h *queryJobStatusAllHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(QueryJobStatusAllRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}

// buildResp builds response.
func (h *queryJobStatusAllHandler) buildResp(resp *interconn.CommonResponse, content map[string]interface{}) {
	status := map[string]interface{}{
		"status": content,
	}

	data, err := structpb.NewStruct(status)
	if err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = bfiacommon.ErrGenerateDataFailed
		return
	}
	resp.Data = data
}
