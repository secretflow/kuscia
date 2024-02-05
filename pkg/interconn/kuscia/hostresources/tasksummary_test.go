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

package hostresources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func makeMockTask(namespace, name string) *v1alpha1.KusciaTask {
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return task
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

func makeMockTaskResource(namespace, name string) *v1alpha1.TaskResource {
	tr := &v1alpha1.TaskResource{
		ObjectMeta: v1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    map[string]string{},
		},
	}

	return tr
}

func TestHandleUpdatedTaskSummary(t *testing.T) {
	kusciaFakeClient := fake.NewSimpleClientset()
	opt := &hostResourcesControllerOptions{
		host:               "alice",
		member:             "bob",
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	ts1 := makeMockTaskSummary("bob", "ts-1", "1")
	ts2 := makeMockTaskSummary("bob", "ts-1", "2")

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
			c.handleUpdatedTaskSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.taskSummaryQueue.Len())
		})
	}
}

func TestSyncTaskSummaryHandler(t *testing.T) {
	ctx := context.Background()

	kts1 := makeMockTaskSummary("bob", "task-1", "1")
	kts1.Spec.Alias = ""
	kts2 := makeMockTaskSummary("bob", "task-2", "2")
	kts2.Spec.JobID = "job-11"
	kts3 := makeMockTaskSummary("bob", "task-3", "3")
	kts4 := makeMockTaskSummary("bob", "task-4", "4")
	hostKusciaFakeClient := fake.NewSimpleClientset(kts1, kts2, kts3, kts4)
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	kj1 := makeMockJob("cross-domain", "job-1")
	kjInformer.Informer().GetStore().Add(kj1)

	kt4 := makeMockTask("cross-domain", "task-4")
	ktInformer.Informer().GetStore().Add(kt4)

	opt := &Options{
		MemberKusciaClient: kusciaFakeClient,
		MemberJobLister:    kjInformer.Lister(),
		MemberTaskLister:   ktInformer.Lister(),
	}
	hrm := NewHostResourcesManager(opt)
	hrm.Register("alice", "bob")
	accessor := hrm.GetHostResourceAccessor("alice", "bob")
	c := accessor.(*hostResourcesController)
	for !c.HasSynced() {
	}

	// taskSummary is empty in host cluster, should return nil
	got := c.syncTaskSummaryHandler(ctx, "bob/task-10")
	assert.Equal(t, nil, got)

	// host taskSummary is invalid, should return nil
	got = c.syncTaskSummaryHandler(ctx, "bob/task-1")
	assert.Equal(t, nil, got)

	// member job doesn't exist, should return error
	got = c.syncTaskSummaryHandler(ctx, "bob/task-2")
	assert.Equal(t, true, got != nil)

	// member task doesn't exist, should return error
	got = c.syncTaskSummaryHandler(ctx, "bob/task-3")
	assert.Equal(t, true, got != nil)

	// member task self cluster party domain id is invalid, should return nil
	got = c.syncTaskSummaryHandler(ctx, "bob/task-4")
	assert.Equal(t, nil, got)

	// member task is invalid, should return nil
	kt4.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	got = c.syncTaskSummaryHandler(ctx, "bob/task-4")
	assert.Equal(t, nil, got)
}

func TestUpdateMemberJobByTaskSummary(t *testing.T) {
	ctx := context.Background()

	hostKusciaFakeClient := fake.NewSimpleClientset()
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kj1 := makeMockJob("cross-domain", "job-1")
	kj1.Spec.Tasks = []v1alpha1.KusciaTaskTemplate{
		{
			Alias:  "task-1",
			TaskID: "",
		},
	}
	kusciaFakeClient := fake.NewSimpleClientset(kj1)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	kjInformer.Informer().GetStore().Add(kj1)

	opt := &Options{
		MemberKusciaClient: kusciaFakeClient,
		MemberJobLister:    kjInformer.Lister(),
	}

	hrm := NewHostResourcesManager(opt)
	hrm.Register("alice", "bob")
	accessor := hrm.GetHostResourceAccessor("alice", "bob")
	c := accessor.(*hostResourcesController)
	for !c.HasSynced() {
	}

	// member job doesn't exist, should return error
	kts := makeMockTaskSummary("bob", "task-2", "1")
	kts.Spec.JobID = "job-22"
	got, err := c.updateMemberJobByTaskSummary(ctx, kts)
	assert.Equal(t, false, got)
	assert.Equal(t, true, err != nil)

	// update job, should return nil
	kts = makeMockTaskSummary("bob", "task-1", "1")
	got, err = c.updateMemberJobByTaskSummary(ctx, kts)
	assert.Equal(t, true, got)
	assert.Equal(t, nil, err)
}

func TestUpdateMemberTask(t *testing.T) {
	// status is Running in task summary, should return false
	kt := makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    v1alpha1.TaskRunning,
		},
	}
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    v1alpha1.TaskRunning,
		},
	}
	domainIDs := []string{"bob"}
	got := updateTaskPartyTaskStatus(kt, kts, domainIDs)
	assert.Equal(t, false, got)

	// status is Failed in task summary, should return false
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    v1alpha1.TaskRunning,
		},
	}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "bob",
			Phase:    v1alpha1.TaskFailed,
		},
	}
	domainIDs = []string{"bob"}
	got = updateTaskPartyTaskStatus(kt, kts, domainIDs)
	assert.Equal(t, true, got)

	// status is updated for non-self party, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "alice",
			Phase:    v1alpha1.TaskRunning,
		},
	}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "alice",
			Phase:    v1alpha1.TaskFailed,
		},
	}
	domainIDs = []string{"bob"}
	got = updateTaskPartyTaskStatus(kt, kts, domainIDs)
	assert.Equal(t, true, got)

	// add new party, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{
		{
			DomainID: "alice",
			Phase:    v1alpha1.TaskFailed,
		},
	}
	domainIDs = []string{"bob"}
	got = updateTaskPartyTaskStatus(kt, kts, domainIDs)
	assert.Equal(t, true, got)
}

func TestUpdateMemberTaskResource(t *testing.T) {
	ctx := context.Background()

	hostKusciaFakeClient := fake.NewSimpleClientset()
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	tr := makeMockTaskResource("bob", "tr-1")
	kusciaFakeClient := fake.NewSimpleClientset(tr)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformer.Informer().GetStore().Add(tr)

	opt := &Options{
		MemberKusciaClient:       kusciaFakeClient,
		MemberTaskResourceLister: trInformer.Lister(),
	}

	hrm := NewHostResourcesManager(opt)
	hrm.Register("alice", "bob")
	accessor := hrm.GetHostResourceAccessor("alice", "bob")
	c := accessor.(*hostResourcesController)
	for !c.HasSynced() {
	}

	domainIDs := []string{"bob"}
	// taskResource doesn't exit, should return nil
	kts := makeMockTaskSummary("bob", "task-2", "1")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName: "tr-2",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseReserving,
				},
			},
		},
	}
	got := c.updateMemberTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)

	// update taskResource, should return nil
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
		"bob": {
			{
				HostTaskResourceName:   "tr-1",
				MemberTaskResourceName: "tr-1",
				TaskResourceStatus: v1alpha1.TaskResourceStatus{
					Phase: v1alpha1.TaskResourcePhaseSchedulable,
				},
			},
		},
	}
	got = c.updateMemberTaskResource(ctx, kts, domainIDs)
	assert.Equal(t, nil, got)
}
