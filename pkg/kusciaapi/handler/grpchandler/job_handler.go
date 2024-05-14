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

package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type jobHandler struct {
	jobService service.IJobService
	kusciaapi.UnimplementedJobServiceServer
}

func NewJobHandler(jobService service.IJobService) kusciaapi.JobServiceServer {
	return &jobHandler{
		jobService: jobService,
	}
}

func (h jobHandler) CreateJob(ctx context.Context, request *kusciaapi.CreateJobRequest) (*kusciaapi.CreateJobResponse, error) {
	res := h.jobService.CreateJob(ctx, request)
	return res, nil
}

func (h jobHandler) QueryJob(ctx context.Context, request *kusciaapi.QueryJobRequest) (*kusciaapi.QueryJobResponse, error) {
	res := h.jobService.QueryJob(ctx, request)
	return res, nil
}

func (h jobHandler) DeleteJob(ctx context.Context, request *kusciaapi.DeleteJobRequest) (*kusciaapi.DeleteJobResponse, error) {
	res := h.jobService.DeleteJob(ctx, request)
	return res, nil
}

func (h jobHandler) StopJob(ctx context.Context, request *kusciaapi.StopJobRequest) (*kusciaapi.StopJobResponse, error) {
	res := h.jobService.StopJob(ctx, request)
	return res, nil
}

func (h jobHandler) SuspendJob(ctx context.Context, request *kusciaapi.SuspendJobRequest) (*kusciaapi.SuspendJobResponse, error) {
	res := h.jobService.SuspendJob(ctx, request)
	return res, nil
}

func (h jobHandler) RestartJob(ctx context.Context, request *kusciaapi.RestartJobRequest) (*kusciaapi.RestartJobResponse, error) {
	res := h.jobService.RestartJob(ctx, request)
	return res, nil
}

func (h jobHandler) CancelJob(ctx context.Context, request *kusciaapi.CancelJobRequest) (*kusciaapi.CancelJobResponse, error) {
	res := h.jobService.CancelJob(ctx, request)
	return res, nil
}

func (h jobHandler) BatchQueryJobStatus(ctx context.Context, request *kusciaapi.BatchQueryJobStatusRequest) (*kusciaapi.BatchQueryJobStatusResponse, error) {
	res := h.jobService.BatchQueryJobStatus(ctx, request)
	return res, nil
}

func (h jobHandler) WatchJob(request *kusciaapi.WatchJobRequest, stream kusciaapi.JobService_WatchJobServer) error {
	eventCh := make(chan *kusciaapi.WatchJobEventResponse, 1)
	defer close(eventCh)
	go func() {
		for e := range eventCh {
			if err := stream.Send(e); err != nil {
				nlog.Errorf("Send job event error: %v", err)
			}
		}
	}()
	err := h.jobService.WatchJob(context.Background(), request, eventCh)
	return err
}

func (h jobHandler) ApproveJob(ctx context.Context, request *kusciaapi.ApproveJobRequest) (*kusciaapi.ApproveJobResponse, error) {
	return h.jobService.ApproveJob(ctx, request), nil
}
