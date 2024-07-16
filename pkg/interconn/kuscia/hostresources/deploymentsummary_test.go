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

func makeMockDeploymentSummary(namespace, name string) *v1alpha1.KusciaDeploymentSummary {
	kd := &v1alpha1.KusciaDeploymentSummary{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return kd
}

func TestHandleUpdatedDeploymentSummary(t *testing.T) {
	t.Parallel()
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

	kds1 := makeMockDeploymentSummary("bob", "kd-1")
	kds2 := makeMockDeploymentSummary("bob", "kd-1")
	kds1.ResourceVersion = "1"
	kds2.ResourceVersion = "2"

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
			name:   "deployment summary is same",
			oldObj: kds1,
			newObj: kds1,
			want:   0,
		},
		{
			name:   "deployment summary is updated",
			oldObj: kds1,
			newObj: kds2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedDeploymentSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.deploymentSummaryQueue.Len())
		})
	}
}

func TestUpdateMemberDeployment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kd := makeMockDeployment("cross-domain", "kd-1")
	kusciaFakeClient := fake.NewSimpleClientset(kd)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	opt := &hostResourcesControllerOptions{
		host:                   "alice",
		member:                 "bob",
		memberKusciaClient:     kusciaFakeClient,
		memberDeploymentLister: kdInformer.Lister(),
	}

	kdInformer.Informer().GetStore().Add(kd)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// deployment doesn't exist, should return error
	kds := makeMockDeploymentSummary("bob", "kd-2")
	got := c.updateMemberDeployment(ctx, kds)
	assert.Equal(t, true, got != nil)

	// self cluster domain ids doesn't exist, should return nil
	kds = makeMockDeploymentSummary("bob", "kd-1")
	got = c.updateMemberDeployment(ctx, kds)
	assert.Equal(t, nil, got)

	// update deployment, should return nil
	kd.Annotations[common.InterConnSelfPartyAnnotationKey] = "bob"
	kd.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"bob": {
			"kd-1": {
				Phase:             v1alpha1.KusciaDeploymentPhaseAvailable,
				Replicas:          1,
				AvailableReplicas: 1,
			},
		},
	}
	kds = makeMockDeploymentSummary("bob", "kd-1")
	kds.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"alice": {
			"kd-1": {
				Phase:             v1alpha1.KusciaDeploymentPhaseAvailable,
				Replicas:          1,
				AvailableReplicas: 1,
			},
		},
	}

	got = c.updateMemberDeployment(ctx, kds)
	assert.Equal(t, nil, got)
}

func TestUpdateDeploymentStatus(t *testing.T) {
	t.Parallel()
	// kd status is empty, should return true
	kd := makeMockDeployment("cross-domain", "kd-1")
	kds := makeMockDeploymentSummary("alice", "kd-1")
	kds.Status.Phase = v1alpha1.KusciaDeploymentPhaseFailed
	kds.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"alice": {
			"kd-1": {
				Phase:             v1alpha1.KusciaDeploymentPhaseFailed,
				Replicas:          1,
				AvailableReplicas: 0,
			},
		},
	}
	got := updateDeploymentStatus(kd, kds, nil)
	assert.Equal(t, true, got)

	// kd partyDeploymentStatuses is not empty, should return true
	kd = makeMockDeployment("cross-domain", "kd-1")
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
	kds.Status.Phase = v1alpha1.KusciaDeploymentPhaseFailed
	kds.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"alice": {
			"kd-1": {
				Phase:             v1alpha1.KusciaDeploymentPhaseAvailable,
				Replicas:          1,
				AvailableReplicas: 0,
			},
		},
	}
	got = updateDeploymentStatus(kd, kds, map[string]struct{}{"bob": struct{}{}})
	assert.Equal(t, true, got)

}
