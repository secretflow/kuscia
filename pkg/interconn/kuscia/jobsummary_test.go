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
)

func makeMockJobSummary(namespace, name string) *v1alpha1.KusciaJobSummary {
	kjs := &v1alpha1.KusciaJobSummary{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return kjs
}

func TestHandleUpdatedJobSummary(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kjs1 := makeMockJobSummary("bob", "job-1")
	kjs2 := makeMockJobSummary("bob", "job-2")
	kjs1.ResourceVersion = "1"
	kjs2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "kjs1",
			newObj: "kjs2",
			want:   0,
		},
		{
			name:   "job summary is same",
			oldObj: kjs1,
			newObj: kjs1,
			want:   0,
		},
		{
			name:   "job summary is updated",
			oldObj: kjs1,
			newObj: kjs2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedJobSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.jobSummaryQueue.Len())
		})
	}
}

func TestUpdateJob(t *testing.T) {
	ctx := context.Background()
	kj := makeMockJob("cross-domain", "job-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kj)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	jobInformer.Informer().GetStore().Add(kj)

	c := &Controller{
		kusciaClient: kusciaFakeClient,
		jobLister:    jobInformer.Lister(),
	}

	// job doesn't exist, should return nil
	kjs := makeMockJobSummary("bob", "job-2")
	got := c.updateJob(ctx, kjs)
	assert.Equal(t, nil, got)

	// job party domain ids is empty, should return nil
	kjs = makeMockJobSummary("bob", "job-1")
	got = c.updateJob(ctx, kjs)
	assert.Equal(t, nil, got)

	// job is updated, should return nil
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	kjs.Spec.Stage = v1alpha1.JobStopStage
	got = c.updateJob(ctx, kjs)
	assert.Equal(t, nil, got)
}

func TestUpdateJobStatusInfo(t *testing.T) {
	domainIDs := []string{"bob"}
	// all status in job and job summary are empty, should return false
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	got := updateJobStatusInfo(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// status is different between job and job summary, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	kjs.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageFailed,
	}
	kjs.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"task-1": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}
	got = updateJobStatusInfo(kj, kjs, domainIDs)
	assert.Equal(t, true, got)
}
