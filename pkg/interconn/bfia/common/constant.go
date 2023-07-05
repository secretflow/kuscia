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

package common

import (
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

const (
	ServerName = "interconn-bfia"
)

const ErrRetries = 3

const (
	InterConnPending = "PENDING"
	InterConnRunning = "RUNNING"
	InterConnSuccess = "SUCCESS"
	InterConnFailed  = "FAILED"
)

var KusciaTaskPhaseToInterConnTaskPhase = map[kusciaapisv1alpha1.KusciaTaskPhase]string{
	kusciaapisv1alpha1.TaskPending:   InterConnPending,
	kusciaapisv1alpha1.TaskRunning:   InterConnRunning,
	kusciaapisv1alpha1.TaskSucceeded: InterConnSuccess,
	kusciaapisv1alpha1.TaskFailed:    InterConnFailed,
}

var InterConnTaskPhaseToKusciaTaskPhase = map[string]kusciaapisv1alpha1.KusciaTaskPhase{
	InterConnPending: kusciaapisv1alpha1.TaskPending,
	InterConnRunning: kusciaapisv1alpha1.TaskRunning,
	InterConnSuccess: kusciaapisv1alpha1.TaskSucceeded,
	InterConnFailed:  kusciaapisv1alpha1.TaskFailed,
}

var KusciaJobPhaseToInterConJobPhase = map[kusciaapisv1alpha1.KusciaJobPhase]string{
	kusciaapisv1alpha1.KusciaJobPending:   InterConnPending,
	kusciaapisv1alpha1.KusciaJobRunning:   InterConnRunning,
	kusciaapisv1alpha1.KusciaJobSucceeded: InterConnSuccess,
	kusciaapisv1alpha1.KusciaJobFailed:    InterConnFailed,
}

const (
	ErrJobAlreadyExist      = "Job already exists"
	ErrJobDoesNotExist      = "job does not exist"
	ErrJobStatusFailed      = "Job status is failed"
	ErrFindJobFailed        = "failed to find the job"
	ErrGenerateDataFailed   = "failed to generate response data"
	ErrTaskDoesNotExist     = "task does not exist"
	ErrFindTaskFailed       = "failed to find the task"
	ErrTaskNameDoesNotExist = "failed to find the task by task name"
)
