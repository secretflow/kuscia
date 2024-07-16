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
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func makeMockDeployment(namespace, name string) *v1alpha1.KusciaDeployment {
	kd := &v1alpha1.KusciaDeployment{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return kd
}

func TestHandleUpdatedDeployment(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kd1 := makeMockDeployment("cross-domain", "kd-1")
	kd2 := makeMockDeployment("cross-domain", "kd-1")
	kd1.ResourceVersion = "1"
	kd2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "kd1",
			newObj: "kd2",
			want:   0,
		},
		{
			name:   "deployment is same",
			oldObj: kd1,
			newObj: kd1,
			want:   0,
		},
		{
			name:   "deployment is updated",
			oldObj: kd1,
			newObj: kd2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedDeployment(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.deploymentQueue.Len())
		})
	}
}

func TestHandleDeletedDeployment(t *testing.T) {
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

	// kd namespace is not cross-domain, deploymentQueue length should be equal to 1
	kd := makeMockDeployment("bob", "kd-1")
	cc.handleDeletedDeployment(kd)
	assert.Equal(t, 1, cc.deploymentQueue.Len())
	cc.deploymentQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), deploymentQueueName)

	// failed to get self cluster is initiator, deploymentQueue length should be equal to 0
	kd = makeMockDeployment("cross-domain", "kd-1")
	cc.handleDeletedDeployment(kd)
	assert.Equal(t, 0, cc.deploymentQueue.Len())

	// enqueue as initiator, deploymentQueue length should be equal to 1
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Annotations[common.InitiatorAnnotationKey] = "alice"
	kd.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "true"
	kd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	cc.handleDeletedDeployment(kd)
	assert.Equal(t, 1, cc.deploymentQueue.Len())
}

func TestDeleteDeploymentCascadedResources(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	kd := makeMockDeployment("bob", "kd-1")
	kdInformer.Informer().GetStore().Add(kd)

	kdsInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeploymentSummaries()
	kds := makeMockDeploymentSummary("bob", "kd-1")
	kdsInformer.Informer().GetStore().Add(kds)

	c := &Controller{
		kusciaClient:            kusciaFakeClient,
		deploymentLister:        kdInformer.Lister(),
		deploymentSummaryLister: kdsInformer.Lister(),
	}

	got := c.deleteDeploymentCascadedResources(context.Background(), "bob", "kd-1")
	assert.Equal(t, nil, got)
}

func TestCreateOrUpdateMirrorDeployments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bobKd := makeMockDeployment("bob", "kd-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(bobKd)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	kdInformer.Informer().GetStore().Add(bobKd)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient:     kusciaFakeClient,
		domainLister:     domainInformer.Lister(),
		deploymentLister: kdInformer.Lister(),
	}

	// failed to party master domain, should return nil
	kd := makeMockDeployment("cross-domain", "kd-1")
	got := c.createOrUpdateMirrorDeployments(ctx, kd)
	assert.Equal(t, nil, got)

	// create mirror deployment, should return nil
	kd = makeMockDeployment("cross-domain", "kd-2")
	kd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateMirrorDeployments(ctx, kd)
	assert.Equal(t, nil, got)

	// update mirror deployment, should return nil
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	kd.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.createOrUpdateMirrorDeployments(ctx, kd)
	assert.Equal(t, nil, got)
}

func TestCreateOrUpdateDeploymentSummary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bobKds := makeMockDeploymentSummary("bob", "kd-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(bobKds)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdsInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeploymentSummaries()
	kdsInformer.Informer().GetStore().Add(bobKds)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainBob := makeMockDomain("bob")
	domainBob.Spec.Role = v1alpha1.Partner
	domainInformer.Informer().GetStore().Add(domainBob)

	c := &Controller{
		kusciaClient:            kusciaFakeClient,
		domainLister:            domainInformer.Lister(),
		deploymentSummaryLister: kdsInformer.Lister(),
	}

	// failed to party master domain, should return nil
	kd := makeMockDeployment("cross-domain", "kd-1")
	got := c.createOrUpdateDeploymentSummary(ctx, kd)
	assert.Equal(t, nil, got)

	// create mirror deployment summary, should return nil
	kd = makeMockDeployment("cross-domain", "kd-2")
	kd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	got = c.createOrUpdateDeploymentSummary(ctx, kd)
	assert.Equal(t, nil, got)

	// update mirror deployment summary, should return nil
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	kd.Annotations[common.InterConnSelfPartyAnnotationKey] = "alice"
	kd.Annotations[common.InitiatorAnnotationKey] = "alice"
	kd.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"bob": {
			"kd-1": {
				Phase:             "Running",
				Replicas:          1,
				AvailableReplicas: 1,
			},
		},
	}
	got = c.createOrUpdateDeploymentSummary(ctx, kd)
	assert.Equal(t, nil, got)
}

func TestProcessDeploymentAsPartner(t *testing.T) {
	t.Parallel()
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
	kds1 := makeMockDeploymentSummary("bob", "job-1")
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kds1)
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
	kd := makeMockDeployment("cross-domain", "kd-1")
	got := c.processDeploymentAsPartner(ctx, kd)
	assert.Equal(t, nil, got)

	// label kuscia party master domain is empty, should return nil
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.processDeploymentAsPartner(ctx, kd)
	assert.Equal(t, nil, got)

	// deployment summary doesn't exist in host cluster, should return nil
	kd = makeMockDeployment("cross-domain", "kd-2")
	kd.Annotations[common.InitiatorAnnotationKey] = "alice"
	kd.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = "bob"
	got = c.processDeploymentAsPartner(ctx, kd)
	assert.Equal(t, nil, got)
}

func TestUpdateDeploymentSummaryPartyStatus(t *testing.T) {
	t.Parallel()
	// kd status is failed and is not equal to kds, should return true
	kd := makeMockDeployment("cross-domain", "kd-1")
	kd.Status.Phase = v1alpha1.KusciaDeploymentPhaseFailed
	kds := makeMockDeploymentSummary("alice", "kd-1")
	got := updateDeploymentSummaryPartyStatus(kd, kds)
	assert.Equal(t, true, got)

	// kd PartyDeploymentStatuses is empty, should return false
	kd = makeMockDeployment("cross-domain", "kd-1")
	got = updateDeploymentSummaryPartyStatus(kd, nil)
	assert.Equal(t, false, got)

	// self cluster party domain ids is empty, should return false
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{}
	got = updateDeploymentSummaryPartyStatus(kd, nil)
	assert.Equal(t, false, got)

	// status is updated, should return true
	kd = makeMockDeployment("cross-domain", "kd-1")
	kd.Annotations = map[string]string{
		common.InterConnSelfPartyAnnotationKey: "alice",
	}
	kd.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"alice": {
			"kd-1": {
				Phase:             v1alpha1.KusciaDeploymentPhaseProgressing,
				Replicas:          1,
				AvailableReplicas: 0,
			},
		},
	}
	kds = makeMockDeploymentSummary("alice", "kd-1")
	got = updateDeploymentSummaryPartyStatus(kd, kds)
	assert.Equal(t, true, got)
}
