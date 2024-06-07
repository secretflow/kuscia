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
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
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

func TestHandleUpdatedTask(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kt1 := makeMockTask("cross-domain", "task-1")
	kt2 := makeMockTask("cross-domain", "task-2")
	kt1.ResourceVersion = "1"
	kt2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "task-1",
			newObj: "task-2",
			want:   0,
		},
		{
			name:   "task is same",
			oldObj: kt1,
			newObj: kt1,
			want:   0,
		},
		{
			name:   "task is updated",
			oldObj: kt1,
			newObj: kt2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedTask(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.taskQueue.Len())
		})
	}
}

func TestHandleDeletedTask(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainBob)

	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)
	cc.domainLister = domainInformer.Lister()

	kt1 := makeMockTask("cross-domain", "task-1")
	kt2 := makeMockTask("cross-domain", "task-2")
	kt2.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "label interconn kuscia party is empty",
			obj:  kt1,
			want: 0,
		},
		{
			name: "label interconn kuscia party include one party",
			obj:  kt2,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleDeletedTask(tt.obj)
			assert.Equal(t, tt.want, cc.taskQueue.Len())
		})
	}
}

func TestDeleteTaskCascadedResources(t *testing.T) {
	t.Parallel()
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	taskSummaryInformer.Informer().GetStore().Add(kts)
	c := &Controller{
		kusciaClient:      kusciaFakeClient,
		taskSummaryLister: taskSummaryInformer.Lister(),
	}

	got := c.deleteTaskCascadedResources(context.Background(), "bob", "task-1")
	assert.Equal(t, nil, got)
}

func TestCreateOrUpdateTaskSummary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()

	kts := makeMockTaskSummary("bob", "task-1", "1")
	taskSummaryInformer.Informer().GetStore().Add(kts)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient:      kusciaFakeClient,
		domainLister:      domainInformer.Lister(),
		taskSummaryLister: taskSummaryInformer.Lister(),
	}

	// interconn kuscia party label is empty, should return nil.
	kt := makeMockTask("bob", "task-1")
	got := c.createOrUpdateTaskSummary(ctx, kt)
	assert.Equal(t, nil, got)

	// create task summary, should return nil.
	domainBob.Spec.Role = v1alpha1.Partner
	kt = makeMockTask("bob", "task-2")
	kt.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateTaskSummary(ctx, kt)
	assert.Equal(t, nil, got)

	// update task summary, should return nil.
	domainBob.Spec.Role = v1alpha1.Partner
	kt = makeMockTask("bob", "task-1")
	kt.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateTaskSummary(ctx, kt)
	assert.Equal(t, nil, got)
}

func TestUpdateTaskSummaryByTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	taskSummaryInformer.Informer().GetStore().Add(kts)

	c := &Controller{
		kusciaClient:      kusciaFakeClient,
		taskSummaryLister: taskSummaryInformer.Lister(),
	}

	// self cluster party domain ids label is empty, should return nil
	kt := makeMockTask("bob", "task-1")
	got := c.updateTaskSummaryByTask(ctx, kt, kts)
	assert.Equal(t, nil, got)

	// task and task summary status phase are different, should return nil
	kt = makeMockTask("bob", "task-1")
	kt.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	kt.Status.Phase = v1alpha1.TaskRunning
	got = c.updateTaskSummaryByTask(ctx, kt, kts)
	assert.Equal(t, nil, got)
}

func TestUpdateTaskSummaryPartyTaskStatus(t *testing.T) {
	t.Parallel()
	domainIDs := []string{"alice", "bob"}

	// party task status is empty in task status, should return false
	kt := makeMockTask("cross-domain", "task-1")
	kts := makeMockTaskSummary("bob", "task-1", "1")
	got := updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, true)
	assert.Equal(t, false, got)

	// party task status is same for task and task summary, should return false
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, true)
	assert.Equal(t, false, got)

	// party task status is empty in task summary but not empty for task, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, true)
	assert.Equal(t, true, got)

	// party task status is different in task summary and task, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	kts = makeMockTaskSummary("joke", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "joke",
		Phase:    v1alpha1.TaskFailed,
	}}
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, true)
	assert.Equal(t, true, got)

	// party task status exist in task but not in task summary, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "alice",
		Phase:    v1alpha1.TaskRunning,
	}}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskFailed,
	}}
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, true)
	assert.Equal(t, true, got)

	// party task status is different in task summary and task for member, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskFailed,
	}}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, false)
	assert.Equal(t, true, got)

	// party task status is same in task summary and task for member, should return true
	kt = makeMockTask("cross-domain", "task-1")
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskFailed,
	}}
	kts = makeMockTaskSummary("bob", "task-1", "1")
	kts.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskFailed,
	}}
	got = updateTaskSummaryPartyTaskStatus(kt, kts, domainIDs, false)
	assert.Equal(t, false, got)
}

func TestProcessTaskAsPartner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kt2 := makeMockTask("cross-domain", "task-2")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kt2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	deploymentInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	taskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	taskResourceInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	domainDataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	domainDataGrantInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	aliceDomain := &v1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{Name: "alice"},
	}
	domainInformer.Informer().GetStore().Add(aliceDomain)

	// prepare hostResourceManager
	kts1 := makeMockTaskSummary("bob", "task-1", "1")
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts1)
	hostresources.GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}
	opt := &hostresources.Options{
		MemberKusciaClient:          kusciaFakeClient,
		MemberDeploymentLister:      deploymentInformer.Lister(),
		MemberJobLister:             jobInformer.Lister(),
		MemberTaskLister:            taskInformer.Lister(),
		MemberTaskResourceLister:    taskResourceInformer.Lister(),
		MemberDomainDataLister:      domainDataInformer.Lister(),
		MemberDomainDataGrantLister: domainDataGrantInformer.Lister(),
	}
	hrm := hostresources.NewHostResourcesManager(opt)
	hrm.Register("alice", "bob")
	for !hrm.GetHostResourceAccessor("alice", "bob").HasSynced() {
	}

	c := &Controller{
		kusciaClient:        kusciaFakeClient,
		hostResourceManager: hrm,
		domainLister:        domainInformer.Lister(),
	}

	// label initiator is empty, should return nil
	kt := makeMockTask("cross-domain", "task-1")
	got := c.processTaskAsPartner(ctx, kt)
	assert.Equal(t, nil, got)

	// label kuscia party master domain is empty, should return nil
	kt = makeMockTask("cross-domain", "task-1")
	kt.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.processTaskAsPartner(ctx, kt)
	assert.Equal(t, nil, got)

	// task summary doesn't exist in host cluster, delete task and should return nil
	kt.Annotations[common.InitiatorAnnotationKey] = "alice"
	kt.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	got = c.processTaskAsPartner(ctx, kt2)
	assert.Equal(t, true, got == nil)
}

func TestUpdateHostTaskSummary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts)
	c := &Controller{
		kusciaClient: kusciaFakeClient,
	}

	// self cluster party domain ids is empty, should return nil
	kt := makeMockTask("bob", "task-1")
	got := c.updateHostTaskSummary(ctx, kusciaFakeClient, kt, kts)
	assert.Equal(t, nil, got)

	// update task summary, should return nil
	kt = makeMockTask("bob", "task-1")
	kt.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	kt.Status.PartyTaskStatus = []v1alpha1.PartyTaskStatus{{
		DomainID: "bob",
		Phase:    v1alpha1.TaskRunning,
	}}
	got = c.updateHostTaskSummary(ctx, kusciaFakeClient, kt, kts)
	assert.Equal(t, nil, got)
}
