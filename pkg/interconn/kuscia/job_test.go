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

	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func makeMockJob(namespace, name string) *v1alpha1.KusciaJob {
	kj := &v1alpha1.KusciaJob{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return kj
}

func TestHandleUpdatedJob(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kj1 := makeMockJob("cross-domain", "job-1")
	kj2 := makeMockJob("cross-domain", "job-2")
	kj1.ResourceVersion = "1"
	kj2.ResourceVersion = "2"

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
			name:   "job is same",
			oldObj: kj1,
			newObj: kj1,
			want:   0,
		},
		{
			name:   "job is updated",
			oldObj: kj1,
			newObj: kj2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedJob(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.jobQueue.Len())
		})
	}
}

func TestHandleDeletedJob(t *testing.T) {
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

	kj1 := makeMockJob("cross-domain", "job-1")
	kj2 := makeMockJob("cross-domain", "job-2")
	kj2.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "true"
	kj2.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	kj2.Annotations[common.InitiatorAnnotationKey] = "alice"

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "label initiator is empty",
			obj:  kj1,
			want: 0,
		},
		{
			name: "label interconn kuscia party include one party",
			obj:  kj2,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleDeletedJob(tt.obj)
			assert.Equal(t, tt.want, cc.jobQueue.Len())
		})
	}
}

func TestDeleteJobCascadedResources(t *testing.T) {
	kj := makeMockJob("bob", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kj, kjs)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	jobInformer.Informer().GetStore().Add(kj)
	jobSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobSummaries()
	jobSummaryInformer.Informer().GetStore().Add(kjs)
	c := &Controller{
		kusciaClient:     kusciaFakeClient,
		jobLister:        jobInformer.Lister(),
		jobSummaryLister: jobSummaryInformer.Lister(),
	}

	got := c.deleteJobCascadedResources(context.Background(), "bob", "job-1")
	assert.Equal(t, nil, got)
}

func TestCreateMirrorJobs(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient: kusciaFakeClient,
		jobLister:    jobInformer.Lister(),
		domainLister: domainInformer.Lister(),
	}

	kj := makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got := c.createMirrorJobs(context.Background(), kj)
	assert.Equal(t, nil, got)
}

func TestBuildMirrorJobs(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	jobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainAlice := makeMockDomain("alice")
	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainAlice)
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient: kusciaFakeClient,
		jobLister:    jobInformer.Lister(),
		domainLister: domainInformer.Lister(),
	}

	// label job stage is stop, should return empty and no error
	kj := makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	kj.Labels[common.LabelJobStage] = "Stop"
	got, err := c.buildMirrorJobs(kj)
	assert.Equal(t, 0, len(got))
	assert.Equal(t, nil, err)

	// master domains is empty, should return empty and error
	kj = makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	kj.Annotations[common.InterConnKusciaPartyAnnotationKey] = "joke"
	got, err = c.buildMirrorJobs(kj)
	assert.Equal(t, 0, len(got))
	assert.Equal(t, nil, err)

	// include one partner, should return not empty and no error
	kj = makeMockJob("cross-domain", "job-1")
	kj.Spec.Tasks = []v1alpha1.KusciaTaskTemplate{{
		Alias:  "task-1",
		TaskID: "task-1",
	}}
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	kj.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got, err = c.buildMirrorJobs(kj)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, nil, err)
}

func TestCreateOrUpdateJobSummary(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	jobSummaryInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobSummaries()
	kjs := makeMockJobSummary("bob", "job-2")
	jobSummaryInformer.Informer().GetStore().Add(kjs)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient:     kusciaFakeClient,
		jobSummaryLister: jobSummaryInformer.Lister(),
		domainLister:     domainInformer.Lister(),
	}

	// party master domains is empty, should return nil
	kj := makeMockJob("cross-domain", "job-1")
	got := c.createOrUpdateJobSummary(ctx, kj)
	assert.Equal(t, nil, got)

	// create job summary, should return nil
	kj = makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateJobSummary(ctx, kj)
	assert.Equal(t, nil, got)

	// update job summary, should return nil
	kj = makeMockJob("cross-domain", "job-2")
	kj.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateJobSummary(ctx, kj)
	assert.Equal(t, nil, got)
}

func TestUpdateJobSummaryStage(t *testing.T) {
	// cancel stage:  don't update
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	kjs.Spec.Stage = v1alpha1.JobCancelStage
	got := updateJobSummaryStage(kj, kjs)
	assert.Equal(t, false, got)
	// not version:  don't update
	kj = makeMockJob("cross-domain", "job-1")
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Spec.Stage = v1alpha1.JobStartStage
	got = updateJobSummaryStage(kj, kjs)
	assert.Equal(t, false, got)
	// new version: update
	kj = makeMockJob("cross-domain", "job-1")
	kj.Labels[common.LabelJobStage] = "Start"
	kj.Labels[common.LabelJobStageVersion] = "1"
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Spec.Stage = v1alpha1.JobCreateStage
	got = updateJobSummaryStage(kj, kjs)
	assert.Equal(t, true, got)
}

func TestUpdateJobSummaryApproveStatus(t *testing.T) {
	domainIDs := []string{"bob"}
	// job approve status is empty, should return false
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	got := updateJobSummaryApproveStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// approve status in job and job summary are same, should return false
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	got = updateJobSummaryApproveStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// approve status in job summary is empty but not empty in job, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	got = updateJobSummaryApproveStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)

	// approve status in job and job summary are different, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobAccepted,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.ApproveStatus = map[string]v1alpha1.JobApprovePhase{
		"bob": v1alpha1.JobRejected,
	}
	got = updateJobSummaryApproveStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)
}

func TestUpdateJobSummaryStageStatus(t *testing.T) {
	domainIDs := []string{"bob"}
	// job stage status is empty, should return false
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	got := updateJobSummaryStageStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// stage status in job and job summary are same, should return false
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageFailed,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageFailed,
	}
	got = updateJobSummaryStageStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// stage status in job summary is empty but not empty in job, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageFailed,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	got = updateJobSummaryStageStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)

	// stage status in job and job summary are different, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobStopStageSucceeded,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.StageStatus = map[string]v1alpha1.JobStagePhase{
		"bob": v1alpha1.JobCreateStageSucceeded,
	}
	got = updateJobSummaryStageStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)
}

func TestUpdateJobSummaryPartyTaskCreateStatus(t *testing.T) {
	domainIDs := []string{"bob"}
	// job party task create status is empty, should return false
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	got := updateJobSummaryPartyTaskCreateStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// party task create status in job and job summary are same, should return false
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}
	got = updateJobSummaryPartyTaskCreateStatus(kj, kjs, domainIDs)
	assert.Equal(t, false, got)

	// party task create status in job summary is empty but not empty in job, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}
	kjs = makeMockJobSummary("bob", "job-1")
	got = updateJobSummaryPartyTaskCreateStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)

	// party task create status in job and job summary are different, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateFailed,
		}},
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.PartyTaskCreateStatus = map[string][]v1alpha1.PartyTaskCreateStatus{
		"bob": {{
			DomainID: "bob",
			Phase:    v1alpha1.KusciaTaskCreateSucceeded,
		}},
	}
	got = updateJobSummaryPartyTaskCreateStatus(kj, kjs, domainIDs)
	assert.Equal(t, true, got)
}

func TestUpdateJobSummaryTaskStatus(t *testing.T) {
	// job task status is empty, should return false
	kj := makeMockJob("cross-domain", "job-1")
	kjs := makeMockJobSummary("bob", "job-1")
	got := updateJobSummaryTaskStatus(kj, kjs)
	assert.Equal(t, false, got)

	// task status in job and job summary are same, should return false
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.TaskStatus = map[string]v1alpha1.KusciaTaskPhase{
		"task-1": v1alpha1.TaskFailed,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.TaskStatus = map[string]v1alpha1.KusciaTaskPhase{
		"task-1": v1alpha1.TaskFailed,
	}
	got = updateJobSummaryTaskStatus(kj, kjs)
	assert.Equal(t, false, got)

	// task status in job and job summary are different, should return true
	kj = makeMockJob("cross-domain", "job-1")
	kj.Status.TaskStatus = map[string]v1alpha1.KusciaTaskPhase{
		"task-1": v1alpha1.TaskFailed,
	}
	kjs = makeMockJobSummary("bob", "job-1")
	kjs.Status.TaskStatus = map[string]v1alpha1.KusciaTaskPhase{
		"task-1": v1alpha1.TaskRunning,
	}
	got = updateJobSummaryTaskStatus(kj, kjs)
	assert.Equal(t, true, got)
}

func TestProcessJobAsPartner(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
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
	kjs1 := makeMockJobSummary("bob", "task-1")
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kjs1)
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
	kj := makeMockJob("cross-domain", "job-1")
	got := c.processJobAsPartner(ctx, kj)
	assert.Equal(t, nil, got)

	// label kuscia party master domain is empty, should return nil
	kj = makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.processJobAsPartner(ctx, kj)
	assert.Equal(t, nil, got)

	// job summary doesn't exist in host cluster, should return nil
	kj = makeMockJob("cross-domain", "job-2")
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	kj.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	got = c.processJobAsPartner(ctx, kj)
	assert.Equal(t, nil, got)
}

func TestUpdateHostJobSummary(t *testing.T) {
	ctx := context.Background()
	kjs := makeMockJobSummary("bob", "job-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kjs)
	c := &Controller{
		kusciaClient: kusciaFakeClient,
	}

	masterDomainID := "alice"
	// self cluster party domain ids is empty, should return nil
	kj := makeMockJob("cross-domain", "job-1")
	got := c.updateHostJobSummary(ctx, masterDomainID, kusciaFakeClient, kj, kjs)
	assert.Equal(t, nil, got)

	// update job summary, should return nil
	kj = makeMockJob("cross-domain", "job-1")
	kj.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	kj.Labels[common.LabelJobStage] = "Start"
	got = c.updateHostJobSummary(ctx, masterDomainID, kusciaFakeClient, kj, kjs)
	assert.Equal(t, nil, got)
}
