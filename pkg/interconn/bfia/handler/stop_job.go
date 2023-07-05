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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

// stopJobHandler defines the handler info for stopping job.
type stopJobHandler struct {
	*ResourcesManager
}

// NewStopJobHandler returns a stopJobHandler instance.
func NewStopJobHandler(rm *ResourcesManager) api.ProtoHandler {
	return &stopJobHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *stopJobHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*interconn.StopJobRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("stop job request type is invalid"))
		return
	}

	if req.JobId == "" {
		errs.AppendErr(fmt.Errorf("parameter job_id can't be empty"))
	}
}

// Handle is used to handle request.
func (h *stopJobHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*interconn.StopJobRequest)
	resp := &interconn.CommonResponse{
		Code: http.StatusOK,
	}

	rawKj, err := h.KjLister.Get(req.JobId)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			rawKj, err = h.KusciaClient.KusciaV1alpha1().KusciaJobs().Get(context.Background(), req.JobId, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				resp.Code = http.StatusBadRequest
				resp.Msg = common.ErrJobDoesNotExist
				return resp
			}
		}

		if err != nil {
			resp.Code = http.StatusInternalServerError
			resp.Msg = common.ErrFindJobFailed
			return resp
		}
	}

	if rawKj.Spec.Stage == kusciaapisv1alpha1.JobStopStage {
		return resp
	}

	kj := rawKj.DeepCopy()
	if err = utilsres.UpdateKusciaJobStage(h.KusciaClient, kj, kusciaapisv1alpha1.JobStopStage, common.ErrRetries); err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = fmt.Sprintf("failed to stop job, %v", err.Error())
		return resp
	}

	h.DeleteJobTaskInfoBy(kj.Name)
	return resp
}

// GetType is used to get request and response type.
func (h *stopJobHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(interconn.StopJobRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}
