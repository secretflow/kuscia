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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

// stopTaskHandler defines the handler info for stopping task.
type stopTaskHandler struct {
	*ResourcesManager
}

// NewStopTaskHandler returns a stopTaskHandler instance.
func NewStopTaskHandler(rm *ResourcesManager) api.ProtoHandler {
	return &stopTaskHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *stopTaskHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*interconn.StopTaskRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("stop task request type is invalid"))
		return
	}

	if req.TaskId == "" {
		errs.AppendErr(fmt.Errorf("parameter task_id can't be empty"))
	}
}

// Handle is used to handle request.
func (h *stopTaskHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*interconn.StopTaskRequest)
	resp := &interconn.CommonResponse{
		Code: http.StatusOK,
	}

	rawKt, err := h.KtLister.Get(req.TaskId)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			rawKt, err = h.KusciaClient.KusciaV1alpha1().KusciaTasks().Get(context.Background(), req.TaskId, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				resp.Code = http.StatusBadRequest
				resp.Msg = common.ErrTaskDoesNotExist
				return resp
			}
		}

		if err != nil {
			resp.Code = http.StatusInternalServerError
			resp.Msg = common.ErrFindTaskFailed
			return resp
		}
	}

	if rawKt.Status.Phase == kusciaapisv1alpha1.TaskFailed || rawKt.Status.Phase == kusciaapisv1alpha1.TaskSucceeded {
		return resp
	}

	kt := rawKt.DeepCopy()
	kt.Status.Phase = kusciaapisv1alpha1.TaskFailed
	cond, _ := utilsres.GetKusciaTaskCondition(&kt.Status, kusciaapisv1alpha1.KusciaTaskCondSuccess, true)
	utilsres.SetKusciaTaskCondition(metav1.Now().Rfc3339Copy(), cond, corev1.ConditionFalse, "TaskIsStopped", "")
	if err = utilsres.UpdateKusciaTaskStatus(h.KusciaClient, rawKt, kt, common.ErrRetries); err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = fmt.Sprintf("failed to stop tsak, %v", err.Error())
		return resp
	}

	h.DeleteTaskJobInfoBy(kt.Name)
	return resp
}

// GetType is used to get request and response type.
func (h *stopTaskHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(interconn.StopTaskRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}
