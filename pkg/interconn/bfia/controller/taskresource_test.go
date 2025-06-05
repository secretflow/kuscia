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
	"fmt"
	"testing"

	gochache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/tests/util"
)

func TestHandleAddedOrDeletedTaskResource(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	cc := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if cc == nil {
		t.Error("new controller failed")
	}
	c := cc.(*Controller)

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "obj type is invalid",
			obj:  "tr1",
			want: 0,
		},
		{
			name: "task resource is valid",
			obj:  tr1,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleAddedOrDeletedTaskResource(tt.obj)
			assert.Equal(t, tt.want, c.trQueue.Len())
		})
	}
}

func TestHandleUpdatedTaskResource(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	cc := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if cc == nil {
		t.Error("new controller failed")
	}
	c := cc.(*Controller)
	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)

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
			c.handleUpdatedTaskResource(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.trQueue.Len())
		})
	}
}

func TestHandleTaskResource(t *testing.T) {
	ctx := context.Background()
	commonAnnotations := map[string]string{
		common.JobIDAnnotationKey:     "job-1",
		common.TaskIDAnnotationKey:    "task-1",
		common.TaskAliasAnnotationKey: "task-1",
	}

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.Labels = map[string]string{
		common.LabelTaskUID: "111",
	}
	tr1.Annotations = commonAnnotations
	tr1.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserving

	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.Annotations = commonAnnotations
	tr2.Labels = map[string]string{
		common.LabelTaskUID: "111",
	}
	tr2.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserved

	tr3 := util.MakeTaskResource("ns1", "tr3", 2, nil)
	tr3.Annotations = commonAnnotations
	tr3.Labels = map[string]string{
		common.LabelTaskUID: "111",
	}
	tr3.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserving

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr1, tr2, tr3)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)
	trInformer.Informer().GetStore().Add(tr3)

	c := &Controller{
		kusciaClient:         kusciaFakeClient,
		ktLister:             ktInformer.Lister(),
		trLister:             trInformer.Lister(),
		inflightRequestCache: gochache.New(inflightRequestCacheExpiration, inflightRequestCacheExpiration),
	}

	tests := []struct {
		name        string
		tr          *kusciaapisv1alpha1.TaskResource
		annotations map[string]string
		wantErr     bool
	}{
		{
			name: "label job id is empty",
			tr:   tr1,
			annotations: map[string]string{
				common.TaskIDAnnotationKey:    "task-1",
				common.TaskAliasAnnotationKey: "task-1",
			},
			wantErr: true,
		},
		{
			name: "label task id is empty",
			tr:   tr1,
			annotations: map[string]string{
				common.JobIDAnnotationKey:     "job-1",
				common.TaskAliasAnnotationKey: "task-1",
			},
			wantErr: true,
		},
		{
			name: "label task alias is empty",
			tr:   tr1,
			annotations: map[string]string{
				common.JobIDAnnotationKey:     "job-1",
				common.TaskAliasAnnotationKey: "task-1",
			},
			wantErr: true,
		},
		{
			name: "existing reserved task resource",
			tr:   tr1,
			annotations: map[string]string{
				common.JobIDAnnotationKey:     "job-1",
				common.TaskIDAnnotationKey:    "task-1",
				common.TaskAliasAnnotationKey: "task-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tr != nil {
				tt.tr.Annotations = tt.annotations
			}
			got := c.handleTaskResource(ctx, tt.tr, fmt.Sprintf("%s/%s", tt.tr.Namespace, tt.tr.Name))
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestSetPartyTaskStatuses(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-1",
			Namespace: common.KusciaCrossDomain,
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			Initiator: "alice",
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "alice",
					AppImageRef: "sf",
				},
				{
					DomainID:    "bob",
					AppImageRef: "sf",
				},
			},
		},
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kt)
	c := &Controller{
		kusciaClient: kusciaFakeClient,
	}

	c.setPartyTaskStatuses(kt, "alice", "", "Running")
	got, err := kusciaFakeClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(context.Background(), kt.Name, metav1.GetOptions{})
	assert.Equal(t, nil, err)
	assert.Equal(t, kusciaapisv1alpha1.TaskRunning, got.Status.PartyTaskStatus[0].Phase)
}

func TestUpdateTaskResourcesStatus(t *testing.T) {
	tr := util.MakeTaskResource("ns1", "tr1", 2, nil)
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr)
	c := &Controller{
		kusciaClient: kusciaFakeClient,
	}
	c.updateTaskResourceStatus(tr, kusciaapisv1alpha1.TaskResourcePhaseReserved, kusciaapisv1alpha1.TaskResourceCondReserved, corev1.ConditionTrue, "")
	got, err := kusciaFakeClient.KusciaV1alpha1().TaskResources(tr.Namespace).Get(context.Background(), tr.Name, metav1.GetOptions{})
	assert.Equal(t, nil, err)
	assert.Equal(t, kusciaapisv1alpha1.TaskResourcePhaseReserved, got.Status.Phase)
}
