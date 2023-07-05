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

package controller

import (
	"context"
	"testing"

	gochache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	pkgcommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
)

func makeKusciaJob(name string, labels map[string]string) *kusciaapisv1alpha1.KusciaJob {
	return &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Initiator: "alice",
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:  "task-1",
					TaskID: "task-1",
					Parties: []kusciaapisv1alpha1.Party{
						{
							DomainID: "alice",
							Role:     "host",
						},
						{
							DomainID: "bob",
							Role:     "guest",
						},
					},
				},
			},
		},
	}
}

func TestHandleAddedOrDeletedKusciaJob(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}

	job := makeKusciaJob("job-1", map[string]string{common.LabelSelfClusterAsInitiator: "true"})

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "obj type is invalid",
			obj:  "job",
			want: 0,
		},
		{
			name: "kuscia job is valid",
			obj:  job,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := c.(*Controller)
			cc.handleAddedOrDeletedKusciaJob(tt.obj)
			assert.Equal(t, tt.want, cc.kjQueue.Len())
		})
	}
}

func TestHandleUpdatedKusciaJob(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}

	kj1 := makeKusciaJob("job-1", map[string]string{common.LabelSelfClusterAsInitiator: "true"})
	kj1.ResourceVersion = "1"
	kj2 := makeKusciaJob("job-2", map[string]string{common.LabelSelfClusterAsInitiator: "true"})
	kj2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "job1",
			newObj: "job2",
			want:   0,
		},
		{
			name:   "kuscia job is same",
			oldObj: kj1,
			newObj: kj1,
			want:   0,
		},
		{
			name:   "kuscia job is updated",
			oldObj: kj1,
			newObj: kj2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := c.(*Controller)
			cc.handleUpdatedKusciaJob(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.kjQueue.Len())
		})
	}
}

func TestProcessJobStatusSyncNextWorkItem(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	kj1 := makeKusciaJob("job-1", nil)
	kj1.Status.Phase = kusciaapisv1alpha1.KusciaJobSucceeded
	kj2 := makeKusciaJob("job-2", nil)
	kj2.Spec.Stage = kusciaapisv1alpha1.JobStopStage
	kjInformer.Informer().GetStore().Add(kj1)
	kjInformer.Informer().GetStore().Add(kj2)

	c := &Controller{
		kusciaClient:      kusciaFakeClient,
		kjLister:          kjInformer.Lister(),
		kjStatusSyncQueue: workqueue.NewNamedDelayingQueue(kusciaJobStatusSyncQueueName),
	}

	tests := []struct {
		name      string
		key       string
		enqueueKj func(key string, queue workqueue.DelayingInterface)
		want      bool
	}{
		{
			name: "kuscia job get failed",
			key:  "job-11",
			enqueueKj: func(key string, queue workqueue.DelayingInterface) {
				queue.Add(key)
			},
			want: true,
		},
		{
			name: "kuscia job status phase is succeeded",
			key:  "job-1",
			enqueueKj: func(key string, queue workqueue.DelayingInterface) {
				queue.Add(key)
			},
			want: true,
		},
		{
			name: "kuscia job stage is not start",
			key:  "job-2",
			enqueueKj: func(key string, queue workqueue.DelayingInterface) {
				queue.Add(key)
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enqueueKj != nil {
				tt.enqueueKj(tt.key, c.kjStatusSyncQueue)
			}
			got := c.processJobStatusSyncNextWorkItem(context.Background())
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSyncJobHandler(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	kj1 := makeKusciaJob("job-1", nil)
	kj2 := makeKusciaJob("job-2", map[string]string{common.LabelSelfClusterAsInitiator: "true"})
	now := metav1.Now()
	kj2.Status.CompletionTime = &now
	kjInformer.Informer().GetStore().Add(kj1)
	kjInformer.Informer().GetStore().Add(kj2)

	c := &Controller{
		kusciaClient:         kusciaFakeClient,
		nsLister:             nsInformer.Lister(),
		kjLister:             kjInformer.Lister(),
		kjStatusSyncQueue:    workqueue.NewNamedDelayingQueue(kusciaJobStatusSyncQueueName),
		inflightRequestCache: gochache.New(inflightRequestCacheExpiration, inflightRequestCacheExpiration),
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "kuscia job get failed",
			key:     "job-11",
			wantErr: false,
		},
		{
			name:    "self cluster is not initiator",
			key:     "job-1",
			wantErr: false,
		},
		{
			name:    "kuscia job has already completed",
			key:     "job-2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncJobHandler(context.Background(), tt.key)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestGetPartiesDomainInfo(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeFakeClient, 0)
	kj := makeKusciaJob("job-1", map[string]string{common.LabelSelfClusterAsInitiator: "true"})
	nsAlice := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}

	nsBob := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
			Labels: map[string]string{
				common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
				common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)},
		},
	}

	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	nsInformer.Informer().GetStore().Add(nsAlice)
	nsInformer.Informer().GetStore().Add(nsBob)

	c := &Controller{
		nsLister: nsInformer.Lister(),
	}

	want := map[string][]string{
		"bob": {"guest"},
	}
	got := c.getPartiesDomainInfo(kj)
	assert.Equal(t, want, got)
}

func TestGetReqDomainIDFromKusciaJob(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kj := makeKusciaJob("job-1", nil)
	want := "alice"
	got := cc.getReqDomainIDFromKusciaJob(kj)
	assert.Equal(t, want, got)
}

func TestUpdateJobStatus(t *testing.T) {
	kj := makeKusciaJob("job-1", nil)
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kj)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	kjInformer.Informer().GetStore().Add(kj)

	now := metav1.Now()
	curKj := kj.DeepCopy()
	curKj.Status.Conditions = []kusciaapisv1alpha1.KusciaJobCondition{
		{
			Type:               kusciaapisv1alpha1.JobValidated,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		},
	}

	curKj.Status.TaskStatus = map[string]kusciaapisv1alpha1.KusciaTaskPhase{
		"task-1": kusciaapisv1alpha1.TaskRunning,
	}

	c := &Controller{
		kusciaClient: kusciaFakeClient,
		kjLister:     kjInformer.Lister(),
	}

	err := c.updateJobStatus(curKj, true, true)
	assert.Nil(t, err)
	assert.Equal(t, kusciaapisv1alpha1.JobValidated, curKj.Status.Conditions[0].Type)
	assert.Equal(t, 1, len(curKj.Status.Conditions))
}

func TestSetKusciaJobTaskStatus(t *testing.T) {
	kj := makeKusciaJob("job-1", nil)
	tests := []struct {
		name   string
		kj     *kusciaapisv1alpha1.KusciaJob
		status map[string]string
		want   bool
	}{
		{
			name: "input status is empty",
			kj:   kj,
			want: false,
		},
		{
			name: "status has one element",
			kj:   kj,
			status: map[string]string{
				"task-1": pkgcommon.InterConnPending,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setKusciaJobTaskStatus(tt.kj, tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}
