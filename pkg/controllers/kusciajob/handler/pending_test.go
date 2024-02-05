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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

// all party create Success
func setJobAllPartyCreateSuccess(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	job.Status.StageStatus = map[string]kusciaapisv1alpha1.JobStagePhase{}
	job.Status.StageStatus["alice"] = kusciaapisv1alpha1.JobCreateStageSucceeded
	job.Status.StageStatus["bob"] = kusciaapisv1alpha1.JobCreateStageSucceeded
}

// one party create failed
func setJobOnePartyCreateFailed(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	job.Status.StageStatus = map[string]kusciaapisv1alpha1.JobStagePhase{}
	job.Status.StageStatus["alice"] = kusciaapisv1alpha1.JobCreateStageSucceeded
	job.Status.StageStatus["bob"] = kusciaapisv1alpha1.JobCreateStageFailed
}

// only one party sucess
func setJobOnlyOnePartyCreateSuccess(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	job.Status.StageStatus = map[string]kusciaapisv1alpha1.JobStagePhase{}
	job.Status.StageStatus["alice"] = kusciaapisv1alpha1.JobCreateStageSucceeded
}

// all party null
func setJobAllPartyCreateNil(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobPending
	job.Status.StageStatus = nil
}

const (
	testCaseAllPartyCreateSuccess = iota
	testCaseOnePartyCreateFailed
	testCaseOnlyOnePartyCreateSuccess
	testCaseAllPartyCreateNull
)

func setJobStageStatus(job *kusciaapisv1alpha1.KusciaJob, testCase int) {
	switch testCase {
	case testCaseAllPartyCreateSuccess:
		setJobAllPartyCreateSuccess(job)
	case testCaseOnePartyCreateFailed:
		setJobOnePartyCreateFailed(job)
	case testCaseOnlyOnePartyCreateSuccess:
		setJobOnlyOnePartyCreateSuccess(job)
	case testCaseAllPartyCreateNull:
		setJobAllPartyCreateNil(job)
	}
}

func TestPendingHandler_HandlePhase(t *testing.T) {
	independentJob := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)

	type fields struct {
		kubeClient   kubernetes.Interface
		kusciaClient versioned.Interface
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
		testCase  int
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantNeedUpdate bool
		wantErr        assert.ErrorAssertionFunc
		wantJobPhase   kusciaapisv1alpha1.KusciaJobPhase
		wantFinalTasks map[string]taskAssertionFunc
	}{
		{
			name: "All party create success should return needUpdate{true} err{nil} phase{running}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseAllPartyCreateSuccess,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobRunning,
		},
		{
			name: "one party reject should return needUpdate{true} err{nil} phase{failed}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseOnePartyCreateFailed,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobFailed,
		},
		{
			name: "only one party accept should return needUpdate{false} err{nil} phase{pending}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseOnlyOnePartyCreateSuccess,
			},
			wantNeedUpdate: false,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "all party nil should return needUpdate{false} err{nil} phase{pending}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseAllPartyCreateNull,
			},
			wantNeedUpdate: false,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobPending,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			setJobStageStatus(tt.args.kusciaJob, tt.args.testCase)

			deps := &Dependencies{
				KusciaClient:     tt.fields.kusciaClient,
				KusciaTaskLister: kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister(),
				NamespaceLister:  nsInformer.Lister(),
				DomainLister:     domainInformer.Lister(),
			}

			h := &PendingHandler{
				JobScheduler: NewJobScheduler(deps),
			}
			gotNeedUpdate, err := h.HandlePhase(tt.args.kusciaJob)
			if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
				return
			}
			assert.Equalf(t, tt.wantNeedUpdate, gotNeedUpdate, "HandlePhase(%v)", tt.args.kusciaJob)
			assert.Equalf(t, tt.wantJobPhase, tt.args.kusciaJob.Status.Phase, "HandlePhase(%v)", tt.args.kusciaJob)
		})
	}
}
