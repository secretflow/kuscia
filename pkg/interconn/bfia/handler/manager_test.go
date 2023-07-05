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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func makeKusciaJob(name string, labels map[string]string, stage kusciaapisv1alpha1.JobStage) *kusciaapisv1alpha1.KusciaJob {
	return &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Stage: stage,
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias: "hetero_logistic_regression_1",
				},
			},
		},
	}
}

func makeKusciaTask(name string, labels map[string]string, phase kusciaapisv1alpha1.KusciaTaskPhase) *kusciaapisv1alpha1.KusciaTask {
	return &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{
			Phase: phase,
			PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
				{
					DomainID: "alice",
					Role:     "host",
					Phase:    "Failed",
				},
			},
			Message: "Failed to run",
		},
	}
}

func TestSyncJobHandler(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	m := &ResourcesManager{
		KusciaClient: kusciaFakeClient,
		KjLister:     kjInformer.Lister(),
		jobTaskInfo:  make(map[string]map[string]struct{}),
		taskJobInfo:  make(map[string]string),
	}

	kj := makeKusciaJob("job-1", nil, kusciaapisv1alpha1.JobStartStage)
	kjInformer.Informer().GetStore().Add(kj)

	m.InsertJob("job-2")
	m.InsertTask("job-2", "kt1")

	tests := []struct {
		name       string
		key        string
		wantLength int
	}{
		{
			name:       "key doesn't exist",
			key:        "job-2",
			wantLength: 0,
		},
		{
			name:       "key is exist",
			key:        "job-1",
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.syncJobHandler(context.Background(), tt.key)
			assert.Equal(t, tt.wantLength, len(m.jobTaskInfo))
		})
	}
}

func TestSyncTaskHandler(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	m := &ResourcesManager{
		KusciaClient: kusciaFakeClient,
		KtLister:     ktInformer.Lister(),
		jobTaskInfo:  make(map[string]map[string]struct{}),
		taskJobInfo:  make(map[string]string),
	}

	kt := makeKusciaTask("task-2", map[string]string{common.LabelJobID: "job-1"}, "")
	ktInformer.Informer().GetStore().Add(kt)

	m.InsertJob("job-1")
	m.InsertTask("job-1", "task-1")

	tests := []struct {
		name       string
		key        string
		wantLength int
	}{
		{
			name:       "key doesn't exist",
			key:        "task-1",
			wantLength: 0,
		},
		{
			name:       "key is exist",
			key:        "task-2",
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.syncTaskHandler(context.Background(), tt.key)
			assert.Equal(t, tt.wantLength, len(m.taskJobInfo))
		})
	}
}

func TestResourceFilter(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "kuscia job label doesn't match",
			obj:  makeKusciaJob("job", nil, kusciaapisv1alpha1.JobStartStage),
			want: false,
		},
		{
			name: "kuscia job label match",
			obj:  makeKusciaJob("job", map[string]string{common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA)}, kusciaapisv1alpha1.JobStartStage),
			want: true,
		},
		{
			name: "kuscia task label doesn't match",
			obj:  makeKusciaTask("task", nil, ""),
			want: false,
		},
		{
			name: "kuscia task label match",
			obj: makeKusciaTask("task", map[string]string{
				common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA),
				common.LabelJobID:                 "job-1",
			}, ""),
			want: true,
		},
		{
			name: "invalid object",
			obj:  "invalid-obj",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resourceFilter(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInsertJob(t *testing.T) {
	m := &ResourcesManager{
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	m.jobTaskInfo = map[string]map[string]struct{}{
		"job-1": {},
	}

	tests := []struct {
		name                      string
		jobID                     string
		expectedJobTaskInfoLength int
	}{
		{
			name:                      "job exist",
			jobID:                     "job-1",
			expectedJobTaskInfoLength: 1,
		},
		{
			name:                      "job doesn't exist",
			jobID:                     "job-2",
			expectedJobTaskInfoLength: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.InsertJob(tt.jobID)
			assert.Equal(t, tt.expectedJobTaskInfoLength, len(m.jobTaskInfo))
		})
	}
}

func TestInsertTask(t *testing.T) {
	m := &ResourcesManager{
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	m.taskJobInfo = map[string]string{
		"task-1": "job-1",
	}

	m.jobTaskInfo = map[string]map[string]struct{}{
		"job-1": {"task-1": struct{}{}},
	}

	tests := []struct {
		name                      string
		jobID                     string
		taskID                    string
		expectedTaskJobInfoLength int
		wantErr                   bool
	}{
		{
			name:                      "job doesn't exist",
			jobID:                     "job-2",
			taskID:                    "task-2",
			expectedTaskJobInfoLength: 1,
			wantErr:                   true,
		},
		{
			name:                      "task exist",
			jobID:                     "job-1",
			taskID:                    "task-1",
			expectedTaskJobInfoLength: 1,
			wantErr:                   false,
		},
		{
			name:                      "task doesn't exist",
			jobID:                     "job-1",
			taskID:                    "task-2",
			expectedTaskJobInfoLength: 2,
			wantErr:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.InsertTask(tt.jobID, tt.taskID)
			assert.Equal(t, tt.expectedTaskJobInfoLength, len(m.taskJobInfo))
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestIsJobExist(t *testing.T) {
	m := &ResourcesManager{
		jobTaskInfo: make(map[string]map[string]struct{}),
	}

	m.jobTaskInfo = map[string]map[string]struct{}{
		"job-1": {"task-1": struct{}{}},
	}

	tests := []struct {
		name  string
		jobID string
		want  bool
	}{
		{
			name:  "job doesn't exist",
			jobID: "job-2",
			want:  false,
		},
		{
			name:  "task exist",
			jobID: "job-1",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.IsJobExist(tt.jobID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTaskExist(t *testing.T) {
	m := &ResourcesManager{
		taskJobInfo: make(map[string]string),
	}

	m.taskJobInfo = map[string]string{
		"task-1": "job-1",
	}

	tests := []struct {
		name   string
		taskID string
		want   bool
	}{
		{
			name:   "task doesn't exist",
			taskID: "task-2",
			want:   false,
		},
		{
			name:   "task exist",
			taskID: "task-1",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.IsTaskExist(tt.taskID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeleteJobTaskInfoBy(t *testing.T) {
	m := &ResourcesManager{
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	m.taskJobInfo = map[string]string{
		"task-1": "job-1",
		"task-2": "job-1",
	}

	m.jobTaskInfo = map[string]map[string]struct{}{
		"job-1": {
			"task-1": struct{}{},
			"task-2": struct{}{},
		},
	}

	tests := []struct {
		name       string
		jobID      string
		wantLength int
	}{
		{
			name:       "clean up job-1",
			jobID:      "job-1",
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.DeleteJobTaskInfoBy(tt.jobID)
			assert.Equal(t, tt.wantLength, len(m.jobTaskInfo))
			assert.Equal(t, tt.wantLength, len(m.taskJobInfo))
		})
	}
}

func TestDeleteTaskJobInfoBy(t *testing.T) {
	m := &ResourcesManager{
		jobTaskInfo: make(map[string]map[string]struct{}),
		taskJobInfo: make(map[string]string),
	}

	m.taskJobInfo = map[string]string{
		"task-1": "job-1",
		"task-2": "job-1",
	}

	m.jobTaskInfo = map[string]map[string]struct{}{
		"job-1": {
			"task-1": struct{}{},
			"task-2": struct{}{},
		},
	}

	tests := []struct {
		name       string
		taskID     string
		wantLength int
	}{
		{
			name:       "clean up task-1",
			taskID:     "task-1",
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.DeleteTaskJobInfoBy(tt.taskID)
			assert.Equal(t, tt.wantLength, len(m.taskJobInfo))
			assert.Equal(t, tt.wantLength, len(m.jobTaskInfo))
		})
	}
}
