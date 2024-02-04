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
)

func makeMockJobSummary(namespace, name string) *v1alpha1.KusciaJobSummary {
	kjs := &v1alpha1.KusciaJobSummary{
		ObjectMeta: v1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    map[string]string{},
		},
	}

	return kjs
}

func TestHandleUpdatedJobSummary(t *testing.T) {
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

	kjs1 := makeMockJobSummary("bob", "job-1")
	kjs1.ResourceVersion = "1"
	kjs2 := makeMockJobSummary("bob", "job-1")
	kjs2.ResourceVersion = "2"

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
			c.handleUpdatedJobSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.jobSummaryQueue.Len())
		})
	}
}

func TestUpdateMemberJobByJobSummary(t *testing.T) {
	ctx := context.Background()
	kj := makeMockJob("cross-domain", "kj-1")
	kj.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	kusciaFakeClient := fake.NewSimpleClientset(kj)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	opt := &hostResourcesControllerOptions{
		host:               "alice",
		member:             "bob",
		memberKusciaClient: kusciaFakeClient,
		memberJobLister:    kjInformer.Lister(),
	}

	kjInformer.Informer().GetStore().Add(kj)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// member job doesn't exist, should return err
	kjs := makeMockJobSummary("bob", "kj-2")
	got := c.updateMemberJobByJobSummary(ctx, kjs)
	assert.Equal(t, true, got != nil)

	// self cluster party domain id is empty in job, should return nil
	kjs = makeMockJobSummary("bob", "kj-1")
	got = c.updateMemberJobByJobSummary(ctx, kjs)
	assert.Equal(t, nil, got)

	// update job, should return nil
	kj.Labels[common.LabelJobStage] = "Create"
	kj.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	kj.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageSucceeded,
	}
	kj.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {
			{
				DomainID: "bob",
				Phase:    v1alpha1.KusciaTaskCreateSucceeded,
			},
		},
	}

	kjs = makeMockJobSummary("bob", "kj-1")
	kjs.Spec.Stage = v1alpha1.JobStopStage
	kjs.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"alice": v1alpha1.JobAccepted,
	}
	kjs.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"alice": v1alpha1.JobCreateStageSucceeded,
	}
	kjs.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"alice": {{
			DomainID: "alice",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}

	got = c.updateMemberJobByJobSummary(ctx, kjs)
	assert.Equal(t, nil, got)
}
