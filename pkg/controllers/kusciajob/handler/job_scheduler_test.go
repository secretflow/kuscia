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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

const (
	KusciaJobForShapeIndependent = "Independent"
	KusciaJobForShapeTree        = "Tree"
	KusciaJobForShapeCycled      = "Cycled"
)

// makeKusciaJob make KusciaJob for test.
// KusciaJobForShapeIndependent: [a,b,c,d]
// KusciaJobForShapeTree: [a,a->b,a->c,c->d]
// KusciaJobForShapeCycled: [d->a,a->b,b->c,c->d]
func makeKusciaJob(shape string, mode kusciaapisv1alpha1.KusciaJobScheduleMode, maxParallelism int,
	tolerableProps *[4]bool) *kusciaapisv1alpha1.KusciaJob {
	parties := []kusciaapisv1alpha1.Party{
		{Role: "client", DomainID: "alice"},
		{Role: "client", DomainID: "bob"},
	}
	kusciaJob := &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secretflow-job",
			Labels: map[string]string{
				common.LabelSelfClusterAsInitiator: "true",
			},
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Initiator:      "alice",
			ScheduleMode:   mode,
			MaxParallelism: &maxParallelism,
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:           "a",
					TaskID:          "a",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config",
					AppImage:        "test-image-1",
					Parties:         parties,
					Dependencies:    []string{},
				},
				{
					Alias:           "b",
					TaskID:          "b",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config2",
					AppImage:        "test-image-2",
					Parties:         parties,
					Dependencies:    []string{},
				},
				{
					Alias:           "c",
					TaskID:          "c",
					Priority:        50,
					TaskInputConfig: "meta://secretflow-1/task-input-config3",
					AppImage:        "test-image-3",
					Parties:         parties,
					Dependencies:    []string{},
				},
				{
					Alias:           "d",
					TaskID:          "d",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config4",
					AppImage:        "test-image-4",
					Parties:         parties,
					Dependencies:    []string{},
				},
			},
		},
		Status: kusciaapisv1alpha1.KusciaJobStatus{
			Conditions: []kusciaapisv1alpha1.KusciaJobCondition{
				{
					Type:   kusciaapisv1alpha1.JobStartInitialized,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   kusciaapisv1alpha1.JobStartSucceeded,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if tolerableProps != nil {
		for i := range kusciaJob.Spec.Tasks {
			kusciaJob.Spec.Tasks[i].Tolerable = &tolerableProps[i]
		}
	}
	switch shape {
	case KusciaJobForShapeIndependent:
		return kusciaJob
	case KusciaJobForShapeTree:
		kusciaJob.Spec.Tasks[1].Dependencies = []string{"a"}
		kusciaJob.Spec.Tasks[2].Dependencies = []string{"a"}
		kusciaJob.Spec.Tasks[3].Dependencies = []string{"c"}
		return kusciaJob
	case KusciaJobForShapeCycled:
		kusciaJob.Spec.Tasks[0].Dependencies = []string{"d"}
		kusciaJob.Spec.Tasks[1].Dependencies = []string{"a"}
		kusciaJob.Spec.Tasks[2].Dependencies = []string{"b"}
		kusciaJob.Spec.Tasks[3].Dependencies = []string{"c"}
		return kusciaJob
	default:
		return nil
	}
}

func Test_kusciaJobValidate(t *testing.T) {
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	nsInformer.Informer().GetStore().Add(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "alice"},
	})

	nsInformer.Informer().GetStore().Add(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "bob"},
	})

	js := &JobScheduler{
		namespaceLister: nsInformer.Lister(),
	}

	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "BestEffort mode task{a,b,c,d} should return want{false}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeIndependent,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} should return want{false}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "BestEffort mode task{[d->a,a->b,b->c,c->d]} should return true",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeCycled,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr(t, js.kusciaJobValidate(tt.args.kusciaJob), "kusciaJobValidate(%v)", tt.args.kusciaJob) {
				return
			}
		})
	}
}

func Test_readyTasksOf(t *testing.T) {
	noDependencies := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	linearDependencies := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	type args struct {
		kusciaJob    *kusciaapisv1alpha1.KusciaJob
		currentTasks map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name string
		args args
		want []kusciaapisv1alpha1.KusciaTaskTemplate
	}{
		{
			name: "BestEffort mode task{a,b,c,d} and succeeded{} should return all {a,b,c,d}",
			args: args{
				kusciaJob:    noDependencies,
				currentTasks: nil,
			},
			want: append(noDependencies.Spec.Tasks[0:2], noDependencies.Spec.Tasks[3], noDependencies.Spec.Tasks[2]),
		},
		{
			name: "BestEffort mode task{a,b,c,d} and succeeded{a,b} should return all {c,d}",
			args: args{
				kusciaJob: noDependencies,
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: noDependencies.Spec.Tasks[2:4],
		},
		{
			name: "BestEffort mode task{a,b,c,d} and succeeded{a} failed{b} should return all {c,d}",
			args: args{
				kusciaJob: noDependencies,
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: noDependencies.Spec.Tasks[2:3],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{} should return all {a,b,c,d}",
			args: args{
				kusciaJob:    linearDependencies,
				currentTasks: nil,
			},
			want: []kusciaapisv1alpha1.KusciaTaskTemplate{linearDependencies.Spec.Tasks[0]},
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a} should return {b,c}",
			args: args{
				kusciaJob: linearDependencies,
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: linearDependencies.Spec.Tasks[1:3],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and failed{a} should return {}",
			args: args{
				kusciaJob: linearDependencies,
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskFailed,
				},
			},
			want: nil,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a} failed{b} should return {c}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
				},
			},
			want: linearDependencies.Spec.Tasks[2:3],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a} failed{c} should return {b}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskFailed,
				},
			},
			want: linearDependencies.Spec.Tasks[1:2],
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a} failed{b} should return {c}",
			args: args{
				kusciaJob: linearDependencies.DeepCopy(),
				currentTasks: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
				},
			},
			want: linearDependencies.Spec.Tasks[2:3],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, readyTasksOf(tt.args.kusciaJob, tt.args.currentTasks),
				"readyTasksOf(%v, %v)", tt.args.kusciaJob, tt.args.currentTasks)
		})
	}
}

func Test_jobStatusPhaseFrom_BestEffort(t *testing.T) {
	type args struct {
		job                   *kusciaapisv1alpha1.KusciaJob
		currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name      string
		args      args
		wantPhase kusciaapisv1alpha1.KusciaJobPhase
	}{
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c-d]} and succeeded{} should Pending",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: nil,
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c-d]} and succeeded{a,b,c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c-d]} and failed{a} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,b,c,d} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c} failed{b} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a} failed{b,c} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c,d} failed{b} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantPhase, jobStatusPhaseFrom(tt.args.job, tt.args.currentSubTasksStatus), "jobStatusPhaseFrom(%v, %v)", tt.args.job, tt.args.currentSubTasksStatus)
		})
	}
}

func Test_jobStatusPhaseFrom_BestEffort_TolerableB(t *testing.T) {
	type args struct {
		job                   *kusciaapisv1alpha1.KusciaJob
		currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name      string
		args      args
		wantPhase kusciaapisv1alpha1.KusciaJobPhase
	}{
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{} should return Pending",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: nil,
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,b',c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c,d} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c} failed{b'} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,b'} failed{c} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,b',c,d} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c,d} failed{b'} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantPhase, jobStatusPhaseFrom(tt.args.job, tt.args.currentSubTasksStatus), "jobStatusPhaseFrom(%v, %v)", tt.args.job, tt.args.currentSubTasksStatus)
		})
	}
}

func Test_jobStatusPhaseFrom_Strict(t *testing.T) {
	type args struct {
		job                   *kusciaapisv1alpha1.KusciaJob
		currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name      string
		args      args
		wantPhase kusciaapisv1alpha1.KusciaJobPhase
	}{
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{} should return Pending",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: nil,
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c} failed{} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,b,c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c,d} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c} failed{b} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,b} failed{c} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,b,c,d} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} and succeeded{a,c,d} failed{b} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantPhase, jobStatusPhaseFrom(tt.args.job, tt.args.currentSubTasksStatus), "jobStatusPhaseFrom(%v, %v)", tt.args.job, tt.args.currentSubTasksStatus)
		})
	}
}

func Test_jobStatusPhaseFrom_Strict_TolerableB(t *testing.T) {
	type args struct {
		job                   *kusciaapisv1alpha1.KusciaJob
		currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name      string
		args      args
		wantPhase kusciaapisv1alpha1.KusciaJobPhase
	}{
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{} should return Pending",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: nil,
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c} failed{b'} should Running",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,b',c} failed{c} should Failed",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskFailed,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,b',c,d} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
		{
			name: "BestEffort mode task{a,[a->b'],[a->c],[c->d]} and succeeded{a,c,d} failed{b'} should Succeeded",
			args: args{
				job: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false}),
				currentSubTasksStatus: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
					"c": kusciaapisv1alpha1.TaskSucceeded,
					"d": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			wantPhase: kusciaapisv1alpha1.KusciaJobSucceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantPhase, jobStatusPhaseFrom(tt.args.job, tt.args.currentSubTasksStatus), "jobStatusPhaseFrom(%v, %v)", tt.args.job, tt.args.currentSubTasksStatus)
		})
	}
}

func Test_willStartTasksOf(t *testing.T) {
	linearDependencies := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	type args struct {
		kusciaJob  *kusciaapisv1alpha1.KusciaJob
		readyTasks []kusciaapisv1alpha1.KusciaTaskTemplate
		status     map[string]kusciaapisv1alpha1.KusciaTaskPhase
	}
	tests := []struct {
		name string
		args args
		want []kusciaapisv1alpha1.KusciaTaskTemplate
	}{
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{2} and succeeded{} should return {a}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{},
			},
			want: linearDependencies.Spec.Tasks[0:1],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{2} and succeeded{a} should return {b,c}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: linearDependencies.Spec.Tasks[1:3],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{2}  and succeeded{a} failed{b} should return {c}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskFailed,
				},
			},
			want: linearDependencies.Spec.Tasks[2:3],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{1} and succeeded{a} failed{b} should return {c}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 1, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: linearDependencies.Spec.Tasks[1:2],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{2} and succeeded{a,b,c} should return {d}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskSucceeded,
					"c": kusciaapisv1alpha1.TaskSucceeded,
				},
			},
			want: linearDependencies.Spec.Tasks[3:4],
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} parallelism{2} and succeeded{a} running{b} should return {c}",
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
				status: map[string]kusciaapisv1alpha1.KusciaTaskPhase{
					"a": kusciaapisv1alpha1.TaskSucceeded,
					"b": kusciaapisv1alpha1.TaskRunning,
				},
			},
			want: linearDependencies.Spec.Tasks[2:3],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.readyTasks = readyTasksOf(tt.args.kusciaJob, tt.args.status)
			assert.Equalf(t, tt.want, willStartTasksOf(tt.args.kusciaJob, tt.args.readyTasks, tt.args.status), "willStartTasksOf(%v, %v, %v)", tt.args.kusciaJob, tt.args.readyTasks, tt.args.status)
		})
	}
}
