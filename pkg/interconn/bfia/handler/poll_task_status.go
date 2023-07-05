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

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

// pollTaskStatusHandler defines the handler info for polling task status.
type pollTaskStatusHandler struct {
	*ResourcesManager
}

// NewPollTaskStatusHandler returns a pollTaskStatusHandler instance.
func NewPollTaskStatusHandler(rm *ResourcesManager) api.ProtoHandler {
	return &pollTaskStatusHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *pollTaskStatusHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*interconn.PollTaskStatusRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("poll task status request type is invalid"))
		return
	}

	if req.TaskId == "" {
		errs.AppendErr(fmt.Errorf("parameter task_id can't be empty"))
	}

	if req.Role == "" {
		errs.AppendErr(fmt.Errorf("parameter role can't be empty"))
	}
}

// Handle is used to handle request.
func (h *pollTaskStatusHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*interconn.PollTaskStatusRequest)
	resp := &interconn.CommonResponse{
		Code: http.StatusOK,
	}

	kt, err := h.KtLister.Get(req.TaskId)
	if k8serrors.IsNotFound(err) {
		if h.IsTaskExist(req.TaskId) {
			h.buildResp(resp, kusciaapisv1alpha1.TaskPending)
			return resp
		}
	}

	if err != nil {
		resp.Code = http.StatusBadRequest
		resp.Msg = common.ErrFindTaskFailed
		return resp
	}

	var roleTaskStatus kusciaapisv1alpha1.KusciaTaskPhase
	for _, task := range kt.Status.PartyTaskStatus {
		if req.Role == task.Role {
			roleTaskStatus = task.Phase
		}
	}

	if roleTaskStatus == "" {
		roleTaskStatus = kusciaapisv1alpha1.TaskPending
	}

	h.buildResp(resp, roleTaskStatus)
	return resp
}

// GetType is used to get request and response type.
func (h *pollTaskStatusHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(interconn.PollTaskStatusRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}

// buildResp builds response.
func (h *pollTaskStatusHandler) buildResp(resp *interconn.CommonResponse, phase kusciaapisv1alpha1.KusciaTaskPhase) {
	status := map[string]interface{}{
		"status": common.KusciaTaskPhaseToInterConnTaskPhase[phase],
	}

	data, err := structpb.NewStruct(status)
	if err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = common.ErrGenerateDataFailed
		return
	}
	resp.Data = data
}
