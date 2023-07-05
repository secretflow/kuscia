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

// startTaskHandler defines the handler info for starting task.
type startTaskHandler struct {
	*ResourcesManager
}

// NewStartTaskHandler returns a startTaskHandler instance.
func NewStartTaskHandler(rm *ResourcesManager) api.ProtoHandler {
	return &startTaskHandler{
		ResourcesManager: rm,
	}
}

// Validate is used to validate request.
func (h *startTaskHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*interconn.StartTaskRequest)
	if !ok {
		errs.AppendErr(fmt.Errorf("start task request type is invalid"))
		return
	}

	if req.JobId == "" {
		errs.AppendErr(fmt.Errorf("parameter job_id can't be empty"))
	}

	if req.TaskId == "" {
		errs.AppendErr(fmt.Errorf("parameter task_id can't be empty"))
	}

	if req.TaskName == "" {
		errs.AppendErr(fmt.Errorf("parameter task_name can't be empty"))
	}
}

// Handle is used to handle request.
func (h *startTaskHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req := request.(*interconn.StartTaskRequest)
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

	if rawKj.Status.Phase == kusciaapisv1alpha1.KusciaJobFailed {
		resp.Code = http.StatusBadRequest
		resp.Msg = getFailedMessageFromKusciaJob(rawKj)
		return resp
	}

	kj := rawKj.DeepCopy()
	found := false
	for _, task := range kj.Spec.Tasks {
		if task.Alias == req.TaskName {
			found = true
			if task.TaskID != "" {
				return resp
			}
			break
		}
	}

	if !found {
		resp.Code = http.StatusBadRequest
		resp.Msg = common.ErrTaskNameDoesNotExist
		return resp
	}

	if err = h.setKusciaJobTaskID(kj, req.TaskId, req.TaskName); err != nil {
		resp.Code = http.StatusInternalServerError
		resp.Msg = err.Error()
		return resp
	}

	h.InsertTask(req.JobId, req.TaskId)
	return resp
}

// GetType is used to get request and response type.
func (h *startTaskHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(interconn.StartTaskRequest{}), reflect.TypeOf(interconn.CommonResponse{})
}

// setKusciaJobTaskID sets task id of kuscia job.
func (h *startTaskHandler) setKusciaJobTaskID(kj *kusciaapisv1alpha1.KusciaJob, taskID, taskAlias string) error {
	update := func(kj *kusciaapisv1alpha1.KusciaJob) {
		for i := range kj.Spec.Tasks {
			if kj.Spec.Tasks[i].Alias == taskAlias {
				kj.Spec.Tasks[i].TaskID = taskID
				return
			}
		}
	}

	hasUpdated := func(kj *kusciaapisv1alpha1.KusciaJob) bool {
		for i := range kj.Spec.Tasks {
			if kj.Spec.Tasks[i].Alias == taskAlias {
				if kj.Spec.Tasks[i].TaskID != "" {
					return true
				}
				return false
			}
		}
		return false
	}

	update(kj)

	return utilsres.UpdateKusciaJob(h.KusciaClient, kj, hasUpdated, update, common.ErrRetries)
}
