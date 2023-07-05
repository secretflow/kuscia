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
	"fmt"
	"net/http"
	"reflect"

	"google.golang.org/protobuf/types/known/structpb"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/interconn/bfia/adapter"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

// createJobHandler defines the handler info for creating job.
type createJobHandler struct {
	*ResourcesManager
}

// NewCreateJobHandler returns a createJobHandler instance.
func NewCreateJobHandler(rm *ResourcesManager) api.ProtoHandler {
	return &createJobHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *createJobHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*interconn.CreateJobRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("create job request type is invalid"))
		return
	}

	if req.JobId == "" {
		errs.AppendErr(fmt.Errorf("parameter job_id can't be empty"))
	}

	if req.Dag == nil {
		errs.AppendErr(fmt.Errorf("parameter dag can't be empty"))
	}

	if req.Config == nil {
		errs.AppendErr(fmt.Errorf("parameter config can't be empty"))
	}
}

// Handle is used to handle request.
func (h *createJobHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*interconn.CreateJobRequest)
	resp := &interconn.CommonResponse{
		Code: http.StatusOK,
	}

	kj, _ := h.KjLister.Get(req.JobId)
	if kj != nil {
		resp.Code = http.StatusBadRequest
		resp.Msg = common.ErrJobAlreadyExist
		return resp
	}

	interConnJobInfo := &adapter.InterConnJobInfo{
		DAG:    req.Dag,
		Config: req.Config,
		JodID:  req.JobId,
		FlowID: req.FlowId,
	}

	kj, err := adapter.GenerateKusciaJobFrom(interConnJobInfo)
	if err != nil {
		resp.Code = http.StatusBadRequest
		resp.Msg = err.Error()
		return resp
	}

	kj, err = h.KusciaClient.KusciaV1alpha1().KusciaJobs().Create(context.Background(), kj, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			h.buildResponseData(resp, common.KusciaJobPhaseToInterConJobPhase[kj.Status.Phase])
			return resp
		}
		resp.Code = http.StatusBadRequest
		resp.Msg = err.Error()
		return resp
	}

	h.buildResponseData(resp, common.InterConnPending)
	return resp
}

// GetType is used to get request and response type.
func (h *createJobHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(interconn.CreateJobRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}

func (h *createJobHandler) buildResponseData(resp *interconn.CommonResponse, status string) {
	var err error
	data := map[string]interface{}{
		"status": status,
	}
	resp.Data, err = structpb.NewStruct(data)
	if err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = err.Error()
	}
}
