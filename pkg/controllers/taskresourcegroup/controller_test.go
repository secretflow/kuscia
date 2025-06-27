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

package taskresourcegroup

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/tests/util"
)

func TestNewController(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	if c == nil {
		t.Error("controller instance should not be nil")
	}
}

func TestHandleAddedTaskResourceGroup(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	trg := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator:          "ns1",
			MinReservedMembers: 3,
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod1",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "object type is invalid",
			obj:  "trg",
			want: 0,
		},
		{
			name: "object is valid",
			obj:  trg,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleAddedTaskResourceGroup(tt.obj)
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestHandleUpdatedTaskResourceGroup(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "trg1",
			ResourceVersion: "1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator: "ns1",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod1",
						},
					},
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhasePending},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "trg1",
			ResourceVersion: "3",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator: "ns1",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod1",
						},
					},
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserving},
	}

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "trg does not update",
			oldObj: trg1,
			newObj: trg1,
			want:   0,
		},
		{
			name:   "trg update",
			oldObj: trg1,
			newObj: trg2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedTaskResourceGroup(tt.oldObj, tt.newObj)
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestHandleDeletedTaskResourceGroup(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	trg := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "trg",
			ResourceVersion: "1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator: "ns1",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns1",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod1",
						},
					},
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{Phase: kusciaapisv1alpha1.TaskResourceGroupPhasePending},
	}

	unKnownObj := cache.DeletedFinalStateUnknown{
		Key: "trg",
		Obj: trg,
	}

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "object type is invalid",
			obj:  "string",
			want: 0,
		},
		{
			name: "object type is DeletedFinalStateUnknown",
			obj:  unKnownObj,
			want: 1,
		},
		{
			name: "object type is trg",
			obj:  trg,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleDeletedTaskResourceGroup(tt.obj)
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestResourceFilter(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	pod1 := st.MakePod().Name("pod1").Namespace("ns1").Obj()
	pod2 := st.MakePod().Name("pod2").Namespace("ns1").
		Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").
		Label(kusciaapisv1alpha1.TaskResourceUID, "2").Obj()

	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "invalid obj type",
			obj:  "test-obj",
			want: false,
		},
		{
			name: "obj label does not match",
			obj:  pod1,
			want: false,
		},
		{
			name: "obj is valid",
			obj:  pod2,
			want: true,
		},
		{
			name: "obj is DeletedFinalStateUnknown type",
			obj: cache.DeletedFinalStateUnknown{
				Key: "ns1/pod2",
				Obj: pod2,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cc.resourceFilter(tt.obj); got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestHandleAddedTaskResource(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	tr := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr",
			Namespace: "ns1",
			Annotations: map[string]string{
				common.TaskResourceGroupAnnotationKey: "trg",
			},
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Initiator: "ns1",
			Pods: []kusciaapisv1alpha1.TaskResourcePod{
				{
					Name: "pod",
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase: kusciaapisv1alpha1.TaskResourcePhaseReserving,
		}}

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "object type is invalid",
			obj:  "tr",
			want: 0,
		},
		{
			name: "object is valid",
			obj:  tr,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleAddedOrDeletedTaskResource(tt.obj)
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestHandleUpdatedTaskResource(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr1",
			Namespace: "ns1",
			Annotations: map[string]string{
				common.TaskResourceGroupAnnotationKey: "trg",
			},
			ResourceVersion: "1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Initiator: "ns1",
			Pods: []kusciaapisv1alpha1.TaskResourcePod{
				{
					Name: "pod",
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase: kusciaapisv1alpha1.TaskResourcePhaseReserving,
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tr2",
			Namespace: "ns1",
			Annotations: map[string]string{
				common.TaskResourceGroupAnnotationKey: "trg",
			},
			ResourceVersion: "2",
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Initiator: "ns1",
			Pods: []kusciaapisv1alpha1.TaskResourcePod{
				{
					Name: "pod",
				},
			},
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase: kusciaapisv1alpha1.TaskResourcePhaseReserving,
		},
	}

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "tr does not update",
			oldObj: tr1,
			newObj: tr1,
			want:   0,
		},
		{
			name:   "tr update",
			oldObj: tr1,
			newObj: tr2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedTaskResource(tt.oldObj, tt.newObj)
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestHandleAddedPod(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.Annotations = map[string]string{
		common.TaskResourceGroupAnnotationKey: "trg1",
	}
	trInformer.Informer().GetStore().Add(tr1)
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)
	cc.trLister = trInformer.Lister()

	pod1 := st.MakePod().Name("pod1").Namespace("ns1").Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Obj()
	cc.handleAddedPod(pod1)
	if cc.trgQueue.Len() != 1 {
		t.Error("trg queue length should be 1")
	}
}

func TestControllerStop(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	cc.Stop()
	if cc.cancel != nil {
		t.Error("controller cancel variable should be nil")
	}
}

func TestSyncHandler(t *testing.T) {
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 3,
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed,
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg2",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 3,
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(trg2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
	trgInformer.Informer().GetStore().Add(trg1)
	trgInformer.Informer().GetStore().Add(trg2)

	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: record.NewFakeRecorder(2),
	})
	cc := c.(*Controller)
	cc.trgLister = trgInformer.Lister()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "trg is not found",
			key:     "trg",
			wantErr: false,
		},
		{
			name:    "trg status phase is TaskResourceGroupPhaseReserveFailed",
			key:     "trg1",
			wantErr: false,
		},
		{
			name:    "trg status phase is TaskResourceGroupPhasePending and content is invalid",
			key:     "trg2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cc.syncHandler(context.Background(), tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}

func TestHandleReserveFailedTrg(t *testing.T) {
	t.Parallel()
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 3,
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseReserveFailed,
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()

	tests := []struct {
		name          string
		queueShutdown bool
		trg           *kusciaapisv1alpha1.TaskResourceGroup
		want          int
	}{
		{
			name:          "trg trgReserveFailedQueue is shutdown",
			queueShutdown: true,
			want:          0,
		},
		{
			name:          "trg is valid",
			queueShutdown: false,
			trg:           trg1,
			want:          1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewController(context.Background(), controllers.ControllerConfig{
				KubeClient:    kubeFakeClient,
				KusciaClient:  kusciaFakeClient,
				EventRecorder: record.NewFakeRecorder(2),
			})
			cc := c.(*Controller)
			if tt.queueShutdown {
				cc.trgReserveFailedQueue.ShutDown()
			} else {
				cc.trgReserveFailedQueue.AddAfter(tt.trg.Name, 0)
			}

			cc.handleReserveFailedTrg()
			got := cc.trgQueue.Len()
			if got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestName(t *testing.T) {
	t.Parallel()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	got := c.Name()
	if got != controllerName {
		t.Error("controller name is wrong")
	}
}
