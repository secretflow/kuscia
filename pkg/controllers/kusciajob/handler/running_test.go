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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

type taskAssertionFunc func(task *kusciaapisv1alpha1.KusciaTask) bool

func taskExistAssertFunc(t *kusciaapisv1alpha1.KusciaTask) bool {
	return t != nil
}
func taskSucceededAssertFunc(t *kusciaapisv1alpha1.KusciaTask) bool {
	return t != nil && t.Status.Phase == kusciaapisv1alpha1.TaskSucceeded
}

const (
	testCaseRunningStart = iota
	testCaseRunningRestart
	testCaseRunningSuspend
	testCaseRunningStop
	testCaseRunningCancel
)

func setRunningJobStage(job *kusciaapisv1alpha1.KusciaJob, testCase int) {
	job.Labels = make(map[string]string)
	job.Labels[common.LabelJobStageTrigger] = "alice"
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
	switch testCase {
	case testCaseRunningStart:
		job.Labels[common.LabelJobStage] = string(kusciaapisv1alpha1.JobStartStage)
	case testCaseRunningRestart:
		job.Labels[common.LabelJobStage] = string(kusciaapisv1alpha1.JobRestartStage)
	case testCaseRunningSuspend:
		job.Labels[common.LabelJobStage] = string(kusciaapisv1alpha1.JobSuspendStage)
	case testCaseRunningStop:
		job.Labels[common.LabelJobStage] = string(kusciaapisv1alpha1.JobStopStage)
	case testCaseRunningCancel:
		job.Labels[common.LabelJobStage] = string(kusciaapisv1alpha1.JobCancelStage)
	default:
		return
	}
}

func TestRunningHandler_HandlePhase(t *testing.T) {
	bestEffortIndependent := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	//bestEffortIndependent.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	bestEffortLinear := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	//bestEffortLinear.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	strictLinear := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, nil)
	//strictLinear.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	strictTolerableBLinear := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeStrict, 2, &[4]bool{false, true, false, false})
	//strictTolerableBLinear.Spec.Stage = kusciaapisv1alpha1.JobStartStage

	type fields struct {
		kubeClient   kubernetes.Interface
		kusciaClient versioned.Interface
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantNeedUpdate bool
		wantErr        assert.ErrorAssertionFunc
		wantJobPhase   kusciaapisv1alpha1.KusciaJobPhase
		wantFinalTasks map[string]taskAssertionFunc
		testcase       int
	}{
		{
			name: "BestEffort mode task{a,b,c,d} maxParallelism{2} and succeeded{a} should return needUpdate{true} err{nil} phase{Running}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					bestEffortIndependent.DeepCopy()),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortIndependent.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"d": taskExistAssertFunc,
			},
		},
		{
			name: "BestEffort mode task{a,b,c,d} maxParallelism{2} and succeeded{a,b,c,d} should return needUpdate{true} err{nil} phase{Succeeded}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("c", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("d", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					bestEffortIndependent.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortIndependent.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobSucceeded,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskSucceededAssertFunc,
				"c": taskSucceededAssertFunc,
				"d": taskSucceededAssertFunc,
			},
		},
		{
			name: "BestEffort mode task{a,b,c,d} maxParallelism{2} and succeeded{a,b,c} should return needUpdate{true} err{nil} phase{Running}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("c", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					bestEffortIndependent.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortIndependent.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskSucceededAssertFunc,
				"c": taskSucceededAssertFunc,
				"d": taskExistAssertFunc,
			},
		},
		{
			name: "BestEffort mode task{a,b,c,d} maxParallelism{2} and succeeded{a,c,d} failed{b} should return needUpdate{true} err{nil} phase{Failed}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskFailed),
					makeTestKusciaTask("c", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("d", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					bestEffortIndependent.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortIndependent.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobFailed,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"c": taskSucceededAssertFunc,
				"d": taskSucceededAssertFunc,
			},
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} maxParallelism{1} and succeeded{a} failed{b} running{c} should return needUpdate{true} err{nil} phase{Running}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", bestEffortLinear.Name, kusciaapisv1alpha1.TaskFailed),
					makeTestKusciaTask("c", bestEffortLinear.Name, kusciaapisv1alpha1.TaskRunning),
					bestEffortLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"c": taskExistAssertFunc,
			},
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} maxParallelism{2} and succeeded{a,b,c,d} needUpdate{true} err{nil} phase{Failed}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskFailed),
					makeTestKusciaTask("c", bestEffortIndependent.Name, kusciaapisv1alpha1.TaskFailed),
					bestEffortLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: bestEffortLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobFailed,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"c": taskExistAssertFunc,
			},
		},
		{
			name: "Strict mode task{a,[a->b],[a->c],[c->d]} maxParallelism{2} and succeeded{a} failed{b} needUpdate{true} err{nil} phase{Failed}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobFailed,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
			},
		},
		{
			name: "Strict mode task{a,[a->b'],[a->c],[c->d]} maxParallelism{2} and succeeded{a} failed{b'} needUpdate{true} err{nil} phase{Running}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"c": taskExistAssertFunc,
			},
		}, {
			name: "test running start",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			wantFinalTasks: map[string]taskAssertionFunc{
				"a": taskSucceededAssertFunc,
				"b": taskExistAssertFunc,
				"c": taskExistAssertFunc,
			},
			testcase: testCaseRunningStart,
		}, {
			name: "test running restart",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
			testcase:       testCaseRunningRestart,
		}, {
			name: "test running suspend",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobSuspended,
			testcase:       testCaseRunningSuspend,
		}, {
			name: "test running cancel",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobCancelled,
			testcase:       testCaseRunningCancel,
		}, {
			name: "test running stop",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(
					makeTestKusciaTask("a", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskSucceeded),
					makeTestKusciaTask("b", strictTolerableBLinear.Name, kusciaapisv1alpha1.TaskFailed),
					strictTolerableBLinear.DeepCopy(),
				),
				kubeClient: kubefake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: strictTolerableBLinear.DeepCopy(),
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobFailed,
			testcase:       testCaseRunningStop,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRunningJobStage(tt.args.kusciaJob, tt.testcase)
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(tt.fields.kusciaClient, 5*time.Minute)
			kubeInformerFactory := informers.NewSharedInformerFactory(tt.fields.kubeClient, 5*time.Minute)
			nsInformer := kubeInformerFactory.Core().V1().Namespaces()
			domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
			aliceNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "alice",
				},
			}
			bobNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bob",
				},
			}
			nsInformer.Informer().GetStore().Add(aliceNs)
			nsInformer.Informer().GetStore().Add(bobNs)
			aliceD := &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "alice",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{},
			}
			bobD := &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bob",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{},
			}
			nsInformer.Informer().GetStore().Add(aliceNs)
			nsInformer.Informer().GetStore().Add(bobNs)
			domainInformer.Informer().GetStore().Add(aliceD)
			domainInformer.Informer().GetStore().Add(bobD)

			deps := &Dependencies{
				KusciaClient:     tt.fields.kusciaClient,
				KusciaTaskLister: kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister(),
				NamespaceLister:  nsInformer.Lister(),
				DomainLister:     domainInformer.Lister(),
			}
			h := &RunningHandler{
				JobScheduler: NewJobScheduler(deps),
			}
			stopCh := make(<-chan struct{})
			go kusciaInformerFactory.Start(stopCh)
			cache.WaitForCacheSync(wait.NeverStop, kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Informer().HasSynced)

			gotNeedUpdate, err := h.HandlePhase(tt.args.kusciaJob)
			if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
				return
			}
			assert.Equalf(t, tt.wantNeedUpdate, gotNeedUpdate, "HandlePhase(%v)", tt.args.kusciaJob)
			assert.Equalf(t, tt.wantJobPhase, tt.args.kusciaJob.Status.Phase, "HandlePhase(%v)", tt.args.kusciaJob)
			if tt.wantFinalTasks != nil {
				subTasks, _ := tt.fields.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).List(context.TODO(), metav1.ListOptions{})
				assert.Equalf(t, true, len(subTasks.Items) == len(tt.wantFinalTasks), "HandlePhase(%v)", tt.args.kusciaJob)
				for _, task := range subTasks.Items {
					assert.Equalf(t, true, tt.wantFinalTasks[task.ObjectMeta.Name](&task), "HandlePhase(%v)", tt.args.kusciaJob)
				}
			}
		})
	}
}

func makeTestKusciaTask(name string, jobName string, phase kusciaapisv1alpha1.KusciaTaskPhase) *kusciaapisv1alpha1.KusciaTask {
	k := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: common.KusciaCrossDomain,
			Annotations: map[string]string{
				common.JobIDAnnotationKey: jobName,
			},
			Labels: map[string]string{
				common.LabelController: LabelControllerValueKusciaJob,
				common.LabelJobUID:     "111",
			},
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			TaskInputConfig: "task input config",
			Parties:         makeMockParties(),
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{
			Phase: phase,
		},
	}
	return k
}

func makeMockParties() []kusciaapisv1alpha1.PartyInfo {
	return []kusciaapisv1alpha1.PartyInfo{
		{
			DomainID:    "domain-a",
			AppImageRef: "test-image-1",
			Role:        "server",
			Template: kusciaapisv1alpha1.PartyTemplate{
				Spec: kusciaapisv1alpha1.PodSpec{
					Containers: []kusciaapisv1alpha1.Container{
						{
							Name:    "container-0",
							Command: []string{"pwd"},
						},
					},
				},
			},
		},
		{
			DomainID:    "domain-b",
			AppImageRef: "test-image-1",
			Role:        "client",
			Template: kusciaapisv1alpha1.PartyTemplate{
				Spec: kusciaapisv1alpha1.PodSpec{
					Containers: []kusciaapisv1alpha1.Container{
						{
							Name:    "container-0",
							Command: []string{"whoami"},
						},
					},
				},
			},
		},
	}
}
