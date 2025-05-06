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

	"github.com/stretchr/testify/assert"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/tests/util"
)

func TestNewController(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	assert.NotNil(t, c)
}

func TestResourceFilter(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	c := &Controller{
		nsLister:     nsInformer.Lister(),
		kusciaClient: kusciaFakeClient,
	}

	kj := makeKusciaJob("job-1", map[string]string{common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA)}, nil)
	kt := makeKusciaTask("task-1", map[string]string{common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA)})
	tr := util.MakeTaskResource("ns1", "tr-1", 2, nil)
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "resource is invalid",
			obj:  "test",
			want: false,
		},
		{
			name: "filter kuscia job",
			obj:  kj,
			want: true,
		},
		{
			name: "filter kuscia task",
			obj:  kt,
			want: true,
		},
		{
			name: "filter task resource",
			obj:  tr,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.resourceFilter(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}

}

func TestGetCacheKeyName(t *testing.T) {
	got := getCacheKeyName(reqTypeCreateJob, resourceTypeKusciaJob, "job-1")
	assert.Equal(t, string(resourceTypeKusciaJob)+"/"+string(reqTypeCreateJob)+"/"+"job-1", got)
}

func TestBuildHostFor(t *testing.T) {
	got := buildHostFor("alice")
	assert.Equal(t, "interconn-scheduler.alice.svc", got)
}

func TestSetPartyTaskStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   *kusciaapisv1alpha1.KusciaTaskStatus
		domainID string
		role     string
		message  string
		phase    kusciaapisv1alpha1.KusciaTaskPhase
		want     bool
	}{
		{
			name: "task phase is same",
			status: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "alice",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskRunning,
					},
				}},
			domainID: "alice",
			role:     "guest",
			phase:    kusciaapisv1alpha1.TaskRunning,
			want:     false,
		},
		{
			name: "task phase is failed",
			status: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "alice",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskFailed,
					},
				}},
			domainID: "alice",
			role:     "guest",
			phase:    kusciaapisv1alpha1.TaskRunning,
			want:     false,
		},
		{
			name: "task phase is succeeded",
			status: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "alice",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskSucceeded,
					},
				}},
			domainID: "alice",
			role:     "guest",
			phase:    kusciaapisv1alpha1.TaskRunning,
			want:     false,
		},
		{
			name: "party task phase changed",
			status: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "alice",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskPending,
					},
				}},
			domainID: "alice",
			role:     "guest",
			phase:    kusciaapisv1alpha1.TaskRunning,
			want:     true,
		},
		{
			name: "add a new party task phase",
			status: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{}},
			domainID: "alice",
			role:     "guest",
			phase:    kusciaapisv1alpha1.TaskRunning,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setPartyTaskStatus(tt.status, tt.domainID, tt.role, tt.message, tt.phase)
			assert.Equal(t, tt.want, got)
		})
	}
}
