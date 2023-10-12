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

//nolint:dulp
package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utils2 "github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IJobService interface {
	CreateJob(ctx context.Context, request *kusciaapi.CreateJobRequest) *kusciaapi.CreateJobResponse
	QueryJob(ctx context.Context, request *kusciaapi.QueryJobRequest) *kusciaapi.QueryJobResponse
	BatchQueryJobStatus(ctx context.Context, request *kusciaapi.BatchQueryJobStatusRequest) *kusciaapi.BatchQueryJobStatusResponse
	StopJob(ctx context.Context, request *kusciaapi.StopJobRequest) *kusciaapi.StopJobResponse
	DeleteJob(ctx context.Context, request *kusciaapi.DeleteJobRequest) *kusciaapi.DeleteJobResponse
	WatchJob(ctx context.Context, request *kusciaapi.WatchJobRequest, event chan<- *kusciaapi.WatchJobEventResponse) error
}

type jobService struct {
	Initiator    string
	kusciaClient kusciaclientset.Interface
}

func NewJobService(config *config.KusciaAPIConfig) IJobService {
	return &jobService{
		Initiator:    config.Initiator,
		kusciaClient: config.KusciaClient,
	}
}

func (h *jobService) CreateJob(ctx context.Context, request *kusciaapi.CreateJobRequest) *kusciaapi.CreateJobResponse {
	// do validate
	// jobID can not be empty
	jobID := request.JobId
	if jobID == "" {
		return &kusciaapi.CreateJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job id can not be empty"),
		}
	}
	// check initiator
	initiator := request.Initiator
	if initiator == "" {
		return &kusciaapi.CreateJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "initiator can not be empty"),
		}
	}
	if h.Initiator != "" && h.Initiator != initiator {
		return &kusciaapi.CreateJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("initiator must be %s in P2P", initiator)),
		}
	}
	// check maxParallelism
	maxParallelism := request.MaxParallelism
	if maxParallelism <= 0 {
		maxParallelism = 1
	}
	// tasks can not be empty
	tasks := request.Tasks
	if len(tasks) == 0 {
		return &kusciaapi.CreateJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "tasks can not be empty"),
		}
	}
	// taskId, parties can not be empty
	for i, task := range tasks {
		if task.Alias == "" {
			return &kusciaapi.CreateJobResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("task alias can not be empty on tasks[%d]", i)),
			}
		}
		parties := task.Parties
		if len(parties) == 0 {
			return &kusciaapi.CreateJobResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("parties can not be empty on tasks[%d]", i)),
			}
		}
		for _, party := range parties {
			if party.DomainId == "" {
				return &kusciaapi.CreateJobResponse{
					Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "party domain id can not be empty"),
				}
			}
		}
	}

	// convert createJobRequest to kuscia job
	kusciaTasks := make([]v1alpha1.KusciaTaskTemplate, len(tasks))
	for i, task := range tasks {
		// build kuscia task parties
		kusicaParties := make([]v1alpha1.Party, len(task.Parties))
		for j, party := range task.Parties {
			kusicaParties[j] = v1alpha1.Party{
				DomainID: party.DomainId,
				Role:     party.Role,
			}
		}
		// build kuscia task
		kusciaTask := v1alpha1.KusciaTaskTemplate{
			TaskID:          task.TaskId,
			Alias:           task.Alias,
			Dependencies:    task.Dependencies,
			AppImage:        task.AppImage,
			TaskInputConfig: task.TaskInputConfig,
			Parties:         kusicaParties,
			Priority:        int(task.Priority),
		}
		kusciaTasks[i] = kusciaTask
	}

	kusciaJob := &v1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobID,
		},
		Spec: v1alpha1.KusciaJobSpec{
			Initiator:      initiator,
			MaxParallelism: utils.IntValue(maxParallelism),
			ScheduleMode:   v1alpha1.KusciaJobScheduleModeBestEffort,
			Tasks:          kusciaTasks,
		},
	}

	// create kuscia job
	_, err := h.kusciaClient.KusciaV1alpha1().KusciaJobs().Create(ctx, kusciaJob, metav1.CreateOptions{})
	if err != nil {
		return &kusciaapi.CreateJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrCreateJob, err.Error()),
		}
	}
	return &kusciaapi.CreateJobResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.CreateJobResponseData{
			JobId: jobID,
		},
	}
}

func (h *jobService) QueryJob(ctx context.Context, request *kusciaapi.QueryJobRequest) *kusciaapi.QueryJobResponse {
	// do validate
	jobID := request.JobId
	if jobID == "" {
		return &kusciaapi.QueryJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job iD can not be empty"),
		}
	}
	// build job status
	kusciaJob, jobStatus, err := h.buildJobStatusByID(ctx, jobID)
	if err != nil {
		return &kusciaapi.QueryJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryJob, err.Error()),
		}
	}
	// build task config
	kusciaJobSpec := kusciaJob.Spec
	kusciaTasks := kusciaJobSpec.Tasks
	taskConfigs := make([]*kusciaapi.TaskConfig, len(kusciaTasks))
	for i, task := range kusciaTasks {
		// build task parties
		taskParties := task.Parties
		parties := make([]*kusciaapi.Party, len(taskParties))
		for j, party := range taskParties {
			parties[j] = &kusciaapi.Party{
				DomainId: party.DomainID,
				Role:     party.Role,
			}
		}

		// build task config
		taskConfig := &kusciaapi.TaskConfig{
			Alias:           task.Alias,
			TaskId:          task.TaskID,
			Parties:         parties,
			AppImage:        task.AppImage,
			Dependencies:    task.Dependencies,
			TaskInputConfig: task.TaskInputConfig,
			Priority:        int32(task.Priority),
		}
		taskConfigs[i] = taskConfig
	}

	// build job response
	jobResponse := &kusciaapi.QueryJobResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.QueryJobResponseData{
			JobId:          jobID,
			Initiator:      kusciaJobSpec.Initiator,
			MaxParallelism: utils.Int32Value(kusciaJobSpec.MaxParallelism),
			Tasks:          taskConfigs,
			Status:         jobStatus,
		},
	}
	return jobResponse
}

func (h *jobService) DeleteJob(ctx context.Context, request *kusciaapi.DeleteJobRequest) *kusciaapi.DeleteJobResponse {
	// do validate
	jobID := request.JobId
	if jobID == "" {
		return &kusciaapi.DeleteJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job id can not be empty"),
		}
	}
	err := h.kusciaClient.KusciaV1alpha1().KusciaJobs().Delete(ctx, jobID, metav1.DeleteOptions{})
	if err != nil {
		return &kusciaapi.DeleteJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrDeleteJob, err.Error()),
		}
	}
	return &kusciaapi.DeleteJobResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.DeleteJobResponseData{
			JobId: jobID,
		},
	}
}

func (h *jobService) StopJob(ctx context.Context, request *kusciaapi.StopJobRequest) *kusciaapi.StopJobResponse {
	// do validate
	jobID := request.JobId
	if jobID == "" {
		return &kusciaapi.StopJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job id can not be empty"),
		}
	}

	job, err := h.kusciaClient.KusciaV1alpha1().KusciaJobs().Get(ctx, jobID, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.StopJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrStopJob, err.Error()),
		}
	}
	job.Spec.Stage = v1alpha1.JobStopStage
	_, err = h.kusciaClient.KusciaV1alpha1().KusciaJobs().Update(ctx, job, metav1.UpdateOptions{})
	if err != nil {
		return &kusciaapi.StopJobResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrStopJob, err.Error()),
		}
	}
	return &kusciaapi.StopJobResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.StopJobResponseData{
			JobId: jobID,
		},
	}
}

func (h *jobService) BatchQueryJobStatus(ctx context.Context, request *kusciaapi.BatchQueryJobStatusRequest) *kusciaapi.BatchQueryJobStatusResponse {
	// do validate
	jobIDs := request.JobIds
	if len(jobIDs) == 0 {
		return &kusciaapi.BatchQueryJobStatusResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job ids can not be empty"),
		}
	}
	for _, jobID := range jobIDs {
		if jobID == "" {
			return &kusciaapi.BatchQueryJobStatusResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "job id can not be empty"),
			}
		}
	}
	// build job status
	jobStatuses := make([]*kusciaapi.JobStatus, len(jobIDs))
	for i, jobID := range jobIDs {
		_, jobStatusDetail, err := h.buildJobStatusByID(ctx, jobID)
		if err != nil {
			return &kusciaapi.BatchQueryJobStatusResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryJobStatus, err.Error()),
			}
		}
		jobStatuses[i] = &kusciaapi.JobStatus{
			JobId:  jobID,
			Status: jobStatusDetail,
		}
	}

	return &kusciaapi.BatchQueryJobStatusResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.BatchQueryJobStatusResponseData{
			Jobs: jobStatuses,
		},
	}

}

func (h *jobService) WatchJob(ctx context.Context, request *kusciaapi.WatchJobRequest, eventCh chan<- *kusciaapi.WatchJobEventResponse) error {
	timeout := request.TimeoutSeconds
	if timeout < 0 {
		return fmt.Errorf("timeout seconds must be greater than or equal to 0")
	}
	var timeoutSeconds *int64
	if request.TimeoutSeconds > 0 {
		timeoutSeconds = &request.TimeoutSeconds
	}
	w, err := h.kusciaClient.KusciaV1alpha1().KusciaJobs().Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		return err
	}
	expectedType := reflect.TypeOf(&v1alpha1.KusciaJob{})

	// Stopping the watcher should be idempotent and if we return from this function there's no way
	// we're coming back in with the same watch interface.
	defer w.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			return errors.New("stop requested")
		case event, ok := <-w.ResultChan():
			if !ok {
				break loop
			}
			if event.Type == watch.Error {
				return apierrors.FromObject(event.Object)
			}
			if expectedType != nil {
				if e, a := expectedType, reflect.TypeOf(event.Object); e != a {
					utilruntime.HandleError(fmt.Errorf("expected type %v, but watch event object had type %v", e, a))
					continue
				}
			}
			job, _ := event.Object.(*v1alpha1.KusciaJob)
			jobStatus, err := h.buildJobStatus(ctx, job)
			if err != nil {
				return err
			}
			switch event.Type {
			case watch.Added:
				eventCh <- &kusciaapi.WatchJobEventResponse{
					Type:   kusciaapi.EventType_ADDED,
					Object: jobStatus,
				}
			case watch.Modified:
				eventCh <- &kusciaapi.WatchJobEventResponse{
					Type:   kusciaapi.EventType_MODIFIED,
					Object: jobStatus,
				}
			case watch.Deleted:
				eventCh <- &kusciaapi.WatchJobEventResponse{
					Type:   kusciaapi.EventType_DELETED,
					Object: jobStatus,
				}
			case watch.Bookmark:
				// A `Bookmark` means watch has synced here, just update the resourceVersion
			default:
				utilruntime.HandleError(fmt.Errorf("unable to understand watch event %#v", event))
			}
		}
	}

	return nil
}

func (h *jobService) buildJobStatusByID(ctx context.Context, jobID string) (*v1alpha1.KusciaJob, *kusciaapi.JobStatusDetail, error) {
	kusciaJob, err := h.kusciaClient.KusciaV1alpha1().KusciaJobs().Get(ctx, jobID, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	jobStatus, err := h.buildJobStatus(ctx, kusciaJob)
	if err != nil {
		return nil, nil, err
	}

	return kusciaJob, jobStatus.Status, nil
}

func (h *jobService) buildJobStatus(ctx context.Context, kusciaJob *v1alpha1.KusciaJob) (*kusciaapi.JobStatus, error) {
	if kusciaJob == nil {
		return nil, fmt.Errorf("kuscia job can not be nil")
	}
	kusciaJobStatus := kusciaJob.Status
	kusciaTasks := kusciaJob.Spec.Tasks
	// build job status
	statusDetail := &kusciaapi.JobStatusDetail{
		State:      string(kusciaJobStatus.Phase),
		ErrMsg:     kusciaJobStatus.Message,
		CreateTime: utils.TimeRfc3339String(&kusciaJob.CreationTimestamp),
		StartTime:  utils.TimeRfc3339String(kusciaJobStatus.StartTime),
		EndTime:    utils.TimeRfc3339String(kusciaJobStatus.CompletionTime),
		Tasks:      make([]*kusciaapi.TaskStatus, 0),
	}

	// build task status
	for _, kt := range kusciaTasks {
		taskID := kt.TaskID
		ts := &kusciaapi.TaskStatus{
			TaskId: taskID,
		}
		if phase, ok := kusciaJobStatus.TaskStatus[taskID]; ok {
			ts.State = string(phase)
			task, err := h.kusciaClient.KusciaV1alpha1().KusciaTasks().Get(ctx, taskID, metav1.GetOptions{})
			if err != nil {
				nlog.Warnf("found task [%s] occurs error: %v", taskID, err.Error())
			} else {
				taskStatus := task.Status
				ts.ErrMsg = taskStatus.Message
				ts.CreateTime = utils.TimeRfc3339String(&task.CreationTimestamp)
				ts.StartTime = utils.TimeRfc3339String(taskStatus.StartTime)
				ts.EndTime = utils.TimeRfc3339String(taskStatus.CompletionTime)
				podStatuses := taskStatus.PodStatuses
				ts.Parties = make([]*kusciaapi.PartyStatus, 0)
				for _, podStatus := range podStatuses {
					ts.Parties = append(ts.Parties, &kusciaapi.PartyStatus{
						DomainId: podStatus.Namespace,
						State:    string(podStatus.PodPhase),
						ErrMsg:   podStatus.TerminationLog,
					})
				}
			}
		}

		statusDetail.Tasks = append(statusDetail.Tasks, ts)
	}
	return &kusciaapi.JobStatus{
		JobId:  kusciaJob.Name,
		Status: statusDetail,
	}, nil
}
