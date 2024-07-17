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

package kuscia

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func makeMockDomain(name string) *v1alpha1.Domain {
	domain := &v1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	}
	return domain
}

func makeMockTaskSummary(namespace, name, rv string) *v1alpha1.KusciaTaskSummary {
	ts := &v1alpha1.KusciaTaskSummary{
		ObjectMeta: v1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
			ResourceVersion: rv,
		},
		Spec: v1alpha1.KusciaTaskSummarySpec{
			Alias: name,
			JobID: "job-1",
		},
	}

	return ts
}

func TestHandleUpdatedTaskSummary(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := fake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	ts1 := makeMockTaskSummary("alice", "ts-1", "1")
	ts2 := makeMockTaskSummary("alice", "ts-1", "2")

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "tr-1",
			newObj: "tr-2",
			want:   0,
		},
		{
			name:   "task summary is same",
			oldObj: ts1,
			newObj: ts1,
			want:   0,
		},
		{
			name:   "task summary is updated",
			oldObj: ts1,
			newObj: ts2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedTaskSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.taskSummaryQueue.Len())
		})
	}
}

func TestUpdateTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kt1 := makeMockTask("cross-domain", "task-1")
	kt1.Annotations[common.TaskSummaryResourceVersionAnnotationKey] = "1"
	kt2 := makeMockTask("cross-domain", "task-2")

	kusciaFakeClient := fake.NewSimpleClientset(kt1, kt2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	ktInformer.Informer().GetStore().Add(kt1)
	ktInformer.Informer().GetStore().Add(kt2)

	ts1 := makeMockTaskSummary("bob", "task-1", "1")
	ts2 := makeMockTaskSummary("bob", "task-2", "2")
	ts2.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    "Pending",
		},
	}
	ts3 := makeMockTaskSummary("bob", "task-3", "3")

	c := &Controller{
		kusciaClient: kusciaFakeClient,
		taskLister:   ktInformer.Lister(),
	}

	tests := []struct {
		name        string
		taskSummary *v1alpha1.KusciaTaskSummary
		domainIDs   []string
		wantErr     bool
	}{
		{
			name:        "taskSummary resource version is not greater than the value in task's label",
			taskSummary: ts1,
			domainIDs:   []string{"bob"},
			wantErr:     false,
		},
		{
			name:        "can't find task by taskSummary",
			taskSummary: ts3,
			domainIDs:   []string{"bob"},
			wantErr:     false,
		},
		{
			name:        "update task by taskSummary",
			taskSummary: ts2,
			domainIDs:   []string{"bob"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := c.updateTask(ctx, tt.taskSummary, tt.domainIDs)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestUpdateTaskPartyStatus(t *testing.T) {
	t.Parallel()
	// partyTaskStatus in taskSummary is empty,  should return false.
	domainIDs := []string{"bob"}
	kt := makeMockTask("cross-domain", "task-1")
	kts := makeMockTaskSummary("bob", "task-1", "1")
	got := updateTaskPartyStatus(kt, kts, domainIDs)
	assert.Equal(t, false, got)

	// partyTaskStatus in taskSummary is not empty taskSummary but in task is empty, should return true.
	kt = makeMockTask("cross-domain", "task-1")
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    "Pending",
		},
	}

	got = updateTaskPartyStatus(kt, kts, domainIDs)
	assert.Equal(t, true, got)
	assert.Equal(t, true, reflect.DeepEqual(kt.Status.PartyTaskStatus, kts.Status.PartyTaskStatus))

	// partyTaskStatus in taskSummary and in task are not empty, should return true.
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    "Pending",
		},
	}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    "Running",
		},
	}
	got = updateTaskPartyStatus(kt, kts, domainIDs)
	assert.Equal(t, true, got)
	assert.Equal(t, true, reflect.DeepEqual(kt.Status.PartyTaskStatus, kts.Status.PartyTaskStatus))

}

func TestUpdateTaskResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tr1 := makeMockTaskResource("bob", "tr-1")
	tr1.Annotations[common.TaskSummaryResourceVersionAnnotationKey] = "1"

	kusciaFakeClient := fake.NewSimpleClientset(tr1)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformer.Informer().GetStore().Add(tr1)

	c := &Controller{
		kusciaClient:       kusciaFakeClient,
		taskResourceLister: trInformer.Lister(),
	}

	// domain doesn't exist in taskResource status, should return nil.
	domainIDs := []string{"alice"}
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName:   "tr-1",
				MemberTaskResourceName: "tr-1",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseReserving,
				},
			},
		},
	}
	got := c.updateTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)

	// taskSummary resource version is not greater than the value in taskResource's label, should return nil.
	domainIDs = []string{"bob"}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName:   "tr-1",
				MemberTaskResourceName: "tr-1",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseReserving,
				},
			},
		},
	}

	got = c.updateTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)

	// taskResource status phase is reserved in taskSummary, should return nil.
	domainIDs = []string{"bob"}
	kts = makeMockTaskSummary("bob", "task-1", "2")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName:   "tr-1",
				MemberTaskResourceName: "tr-1",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseReserved,
				},
			},
		},
	}

	got = c.updateTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)

	// taskResource status phase is failed in taskSummary, should return nil.
	domainIDs = []string{"bob"}
	kts = makeMockTaskSummary("bob", "task-1", "2")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName:   "tr-1",
				MemberTaskResourceName: "tr-1",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseFailed,
				},
			},
		},
	}

	got = c.updateTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)
}
