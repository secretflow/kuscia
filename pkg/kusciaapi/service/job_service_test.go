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

//nolint:dupl
package service

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	consts "github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func TestCreateJob(t *testing.T) {
	res := kusciaAPIJS.CreateJob(context.Background(), &kusciaapi.CreateJobRequest{
		JobId:     kusciaAPIJS.jobID,
		Initiator: "alice",
		Tasks:     kusciaAPIJS.tasks,
	})
	assert.Equal(t, res.Data.JobId, kusciaAPIJS.jobID)
}

func TestQueryJob(t *testing.T) {
	queryJobResponse := kusciaAPIJS.QueryJob(context.Background(), &kusciaapi.QueryJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, queryJobResponse.Data.JobId, kusciaAPIJS.jobID)
	assert.Equal(t, len(queryJobResponse.Data.Tasks), len(kusciaAPIJS.tasks))
}

func TestSuspendJob(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, consts.AuthRole, consts.AuthRoleMaster)
	ctx = context.WithValue(ctx, consts.SourceDomainKey, "alice")
	kusciaAPIJS.CreateJob(ctx, &kusciaapi.CreateJobRequest{
		JobId:     kusciaAPIJS.jobID,
		Initiator: "alice",
		Tasks:     kusciaAPIJS.tasks,
	})
	res := kusciaAPIJS.SuspendJob(ctx, &kusciaapi.SuspendJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Status.Code == 0, false)
	kj, err := kusciaClient.KusciaV1alpha1().KusciaJobs("cross-domain").Get(ctx, kusciaAPIJS.jobID, metav1.GetOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	kj.Status.Phase = v1alpha1.KusciaJobRunning
	kj, err = kusciaClient.KusciaV1alpha1().KusciaJobs("cross-domain").UpdateStatus(ctx, kj, metav1.UpdateOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	res = kusciaAPIJS.SuspendJob(ctx, &kusciaapi.SuspendJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Data.JobId, kusciaAPIJS.jobID)
}

func TestRestartJob(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, consts.AuthRole, consts.AuthRoleMaster)
	ctx = context.WithValue(ctx, consts.SourceDomainKey, "alice")
	res := kusciaAPIJS.RestartJob(ctx, &kusciaapi.RestartJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Status.Code == 0, false)
	kj, err := kusciaClient.KusciaV1alpha1().KusciaJobs("cross-domain").Get(ctx, kusciaAPIJS.jobID, metav1.GetOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	kj.Status.Phase = v1alpha1.KusciaJobFailed
	kj, err = kusciaClient.KusciaV1alpha1().KusciaJobs("cross-domain").UpdateStatus(ctx, kj, metav1.UpdateOptions{})
	if err != nil {
		t.Error(err.Error())
	}
	res = kusciaAPIJS.RestartJob(ctx, &kusciaapi.RestartJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Data.JobId, kusciaAPIJS.jobID)
}

func TestStopJob(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, consts.AuthRole, consts.AuthRoleMaster)
	ctx = context.WithValue(ctx, consts.SourceDomainKey, "alice")
	res := kusciaAPIJS.StopJob(ctx, &kusciaapi.StopJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Data.JobId, kusciaAPIJS.jobID)
}

func TestCancelJob(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, consts.AuthRole, consts.AuthRoleMaster)
	ctx = context.WithValue(ctx, consts.SourceDomainKey, "alice")
	res := kusciaAPIJS.CancelJob(ctx, &kusciaapi.CancelJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, res.Data.JobId, kusciaAPIJS.jobID)
}

func TestBatchQueryJob(t *testing.T) {
	batchResponse := kusciaAPIJS.BatchQueryJobStatus(context.Background(), &kusciaapi.BatchQueryJobStatusRequest{
		JobIds: []string{kusciaAPIJS.jobID},
	})
	assert.Equal(t, len(batchResponse.Data.Jobs), 1)
}

func TestDeleteJob(t *testing.T) {
	deleteRes := kusciaAPIJS.DeleteJob(context.Background(), &kusciaapi.DeleteJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, deleteRes.Data.JobId, kusciaAPIJS.jobID)
	queryRes := kusciaAPIJS.QueryJob(context.Background(), &kusciaapi.QueryJobRequest{
		JobId: kusciaAPIJS.jobID,
	})
	assert.Equal(t, queryRes.Status.Code, int32(errorcode.ErrorCode_KusciaAPIErrQueryJob))
}

func TestBuildScheduleConfigForKusciaTask(t *testing.T) {
	tests := []struct {
		name string
		sc   *kusciaapi.ScheduleConfig
		want *v1alpha1.ScheduleConfig
	}{
		{
			name: "input is empty",
			sc:   nil,
			want: nil,
		},
		{
			name: "input is not empty",
			sc: &kusciaapi.ScheduleConfig{
				TaskTimeoutSeconds:                  0,
				ResourceReservedSeconds:             0,
				ResourceReallocationIntervalSeconds: 0,
			},
			want: &v1alpha1.ScheduleConfig{
				LifecycleSeconds:        300,
				ResourceReservedSeconds: 30,
				RetryIntervalSeconds:    30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScheduleConfigForKusciaTask(tt.sc)
			if tt.want == nil {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Equal(t, tt.want.LifecycleSeconds, got.LifecycleSeconds)
				assert.Equal(t, tt.want.ResourceReservedSeconds, got.ResourceReservedSeconds)
				assert.Equal(t, tt.want.RetryIntervalSeconds, got.RetryIntervalSeconds)
			}
		})
	}
}

func TestBuildScheduleConfigForKusciaAPI(t *testing.T) {
	tests := []struct {
		name string
		sc   *v1alpha1.ScheduleConfig
		want *kusciaapi.ScheduleConfig
	}{
		{
			name: "input is empty",
			sc:   nil,
			want: nil,
		},
		{
			name: "input is not empty",
			sc: &v1alpha1.ScheduleConfig{
				LifecycleSeconds:        0,
				ResourceReservedSeconds: 0,
				RetryIntervalSeconds:    0,
			},
			want: &kusciaapi.ScheduleConfig{
				TaskTimeoutSeconds:                  300,
				ResourceReservedSeconds:             30,
				ResourceReallocationIntervalSeconds: 30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScheduleConfigForKusciaAPI(tt.sc)
			if tt.want == nil {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Equal(t, tt.want.TaskTimeoutSeconds, got.TaskTimeoutSeconds)
				assert.Equal(t, tt.want.ResourceReservedSeconds, got.ResourceReservedSeconds)
				assert.Equal(t, tt.want.ResourceReallocationIntervalSeconds, got.ResourceReallocationIntervalSeconds)
			}
		})
	}
}

func TestAggregateErrorMessage(t *testing.T) {
	// Create jobService instance for testing
	js := &jobService{}

	tests := []struct {
		name     string
		jobMsg   string
		jobPhase v1alpha1.KusciaJobPhase
		tasks    []*kusciaapi.TaskStatus
		want     string
	}{
		{
			name:     "job message not empty should return original message",
			jobMsg:   "original job error",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks:    []*kusciaapi.TaskStatus{},
			want:     "original job error",
		},
		{
			name:     "job phase not failed should return original message",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobRunning,
			tasks:    []*kusciaapi.TaskStatus{},
			want:     "",
		},
		{
			name:     "no failed tasks should return original message",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks: []*kusciaapi.TaskStatus{
				{
					Alias: "task1",
					State: kusciaapi.JobState_Succeeded.String(),
					Parties: []*kusciaapi.PartyStatus{
						{DomainId: "alice", State: kusciaapi.JobState_Succeeded.String()},
					},
				},
			},
			want: "",
		},
		{
			name:     "single task single party failed",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks: []*kusciaapi.TaskStatus{
				{
					Alias: "task1",
					State: kusciaapi.JobState_Failed.String(),
					Parties: []*kusciaapi.PartyStatus{
						{
							DomainId: "alice",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "connection timeout",
						},
					},
				},
			},
			want: "Task[task1]-Party[alice]: connection timeout",
		},
		{
			name:     "multiple tasks multiple parties failed",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks: []*kusciaapi.TaskStatus{
				{
					Alias: "task1",
					State: kusciaapi.JobState_Failed.String(),
					Parties: []*kusciaapi.PartyStatus{
						{
							DomainId: "alice",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "connection timeout",
						},
						{
							DomainId: "bob",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "resource insufficient",
						},
					},
				},
				{
					Alias: "task2",
					State: kusciaapi.JobState_Failed.String(),
					Parties: []*kusciaapi.PartyStatus{
						{
							DomainId: "charlie",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "data validation error",
						},
					},
				},
			},
			want: "Task[task1]-Party[alice]: connection timeout; Task[task1]-Party[bob]: resource insufficient; Task[task2]-Party[charlie]: data validation error",
		},
		{
			name:     "mixed party states only failed parties reported",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks: []*kusciaapi.TaskStatus{
				{
					Alias: "task1",
					State: kusciaapi.JobState_Failed.String(),
					Parties: []*kusciaapi.PartyStatus{
						{
							DomainId: "alice",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "connection timeout",
						},
						{
							DomainId: "bob",
							State:    kusciaapi.JobState_Succeeded.String(),
							ErrMsg:   "", // Successful party should not have error message
						},
					},
				},
			},
			want: "Task[task1]-Party[alice]: connection timeout",
		},
		{
			name:     "failed party with empty error message should be ignored",
			jobMsg:   "",
			jobPhase: v1alpha1.KusciaJobFailed,
			tasks: []*kusciaapi.TaskStatus{
				{
					Alias: "task1",
					State: kusciaapi.JobState_Failed.String(),
					Parties: []*kusciaapi.PartyStatus{
						{
							DomainId: "alice",
							State:    kusciaapi.JobState_Failed.String(),
							ErrMsg:   "", // Empty error message should be ignored
						},
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := js.aggregateErrorMessage(tt.jobMsg, tt.jobPhase, tt.tasks)
			assert.Equal(t, tt.want, got)
		})
	}
}
