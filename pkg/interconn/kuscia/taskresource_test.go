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

func makeMockTaskResource(namespace, name string) *v1alpha1.TaskResource {
	tr := &v1alpha1.TaskResource{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return tr
}

func TestHandleUpdatedTaskResource(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tr1 := makeMockTaskResource("bob", "tr-1")
	tr2 := makeMockTaskResource("bob", "tr-2")
	tr1.ResourceVersion = "1"
	tr2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "tr1",
			newObj: "tr2",
			want:   0,
		},
		{
			name:   "task resource is same",
			oldObj: tr1,
			newObj: tr1,
			want:   0,
		},
		{
			name:   "task resource is updated",
			oldObj: tr1,
			newObj: tr2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedTaskResource(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.taskResourceQueue.Len())
		})
	}
}

func TestUpdateTaskSummaryByTaskResource(t *testing.T) {
	ctx := context.Background()
	kts := makeMockTaskSummary("bob", "task-1", "1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	taskSummaryInformer.Informer().GetStore().Add(kts)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient:      kusciaFakeClient,
		domainLister:      domainInformer.Lister(),
		taskSummaryLister: taskSummaryInformer.Lister(),
	}

	// task id in label is empty, should return nil.
	tr := makeMockTaskResource("bob", "tr-1")
	got := c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// domain doesn't exist, should return not nil.
	tr = makeMockTaskResource("alice", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, true, got != nil)

	// domain role is not partner, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// task doesn't exist, should return error.
	tr = makeMockTaskResource("bob", "tr-1")
	domainBob.Spec.Role = v1alpha1.Partner
	tr.Annotations[common.TaskIDAnnotationKey] = "task-2"
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, true, got != nil)

	// taskResource status phase is reserving and empty in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseReserving
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// taskResource status phase is reserving and failed in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseReserving
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
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// taskResource status phase is failed and failed in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseFailed
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
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// taskResource status phase is failed and reserving in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseFailed
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
	got = c.updateTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)
}

func TestUpdateHostTaskSummaryByTaskResource(t *testing.T) {
	ctx := context.Background()

	kts := makeMockTaskSummary("bob", "task-1", "1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	deploymentInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	taskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	taskResourceInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	domainDataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	domainDataGrantInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()

	taskSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTaskSummaries()
	taskSummaryInformer.Informer().GetStore().Add(kts)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainInformer.Informer().GetStore().Add(domainBob)

	// prepare hostResourceManager
	kts1 := makeMockTaskSummary("bob", "task-1", "1")
	kts2 := makeMockTaskSummary("bob", "task-2", "2")
	kts2.Status.ResourceStatus = map[string][]*v1alpha1.TaskSummaryResourceStatus{
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
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kts1, kts2)
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
		domainLister:        domainInformer.Lister(),
		taskSummaryLister:   taskSummaryInformer.Lister(),
		hostResourceManager: hrm,
	}
	// task id in label is empty, should return nil.
	tr := makeMockTaskResource("bob", "tr-1")
	got := c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// initiator doesn't exist, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// master domain id doesn't exist, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// taskSummary doesn't exist in host cluster, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-3"
	tr.Annotations[common.InitiatorAnnotationKey] = "alice"
	tr.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// task resource status phase is empty, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-1"
	tr.Annotations[common.InitiatorAnnotationKey] = "alice"
	tr.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// task resource status phase is reserving but resource version is less than the value in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-2"
	tr.Annotations[common.InitiatorAnnotationKey] = "alice"
	tr.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseReserving
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)

	// task resource status phase is reserving and resource version is greater than the value in taskSummary, should return nil.
	tr = makeMockTaskResource("bob", "tr-1")
	tr.Annotations[common.TaskIDAnnotationKey] = "task-2"
	tr.Annotations[common.InitiatorAnnotationKey] = "alice"
	tr.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	tr.ResourceVersion = "3"
	tr.Status.Phase = v1alpha1.TaskResourcePhaseReserved
	got = c.updateHostTaskSummaryByTaskResource(ctx, tr)
	assert.Equal(t, nil, got)
}
