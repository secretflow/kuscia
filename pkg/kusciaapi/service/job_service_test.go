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

package service

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
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
	assert.Equal(t, queryRes.Status.Code, int32(errorcode.ErrQueryJob))
}
