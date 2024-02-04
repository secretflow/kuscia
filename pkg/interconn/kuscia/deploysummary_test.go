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

func makeMockDeploymentSummary(namespace, name string) *v1alpha1.KusciaDeploymentSummary {
	kds := &v1alpha1.KusciaDeploymentSummary{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	return kds
}

func TestHandleUpdatedDeploymentSummary(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	kds1 := makeMockDeploymentSummary("cross-domain", "kd-1")
	kds2 := makeMockDeploymentSummary("cross-domain", "kd-2")
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
			oldObj: "kd-1",
			newObj: "kd-2",
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
			cc.handleUpdatedDeploymentSummary(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, cc.deploymentSummaryQueue.Len())
		})
	}
}

func TestUpdateDeployment(t *testing.T) {
	ctx := context.Background()
	kd := makeMockDeployment("cross-domain", "kd-1")
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(kd)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	kdInformer.Informer().GetStore().Add(kd)
	c := &Controller{
		kusciaClient:     kusciaFakeClient,
		deploymentLister: kdInformer.Lister(),
	}

	// deployment doesn't exist, should return nil
	kds := makeMockDeploymentSummary("cross-domain", "kd-2")
	got := c.updateDeployment(ctx, kds)
	assert.Equal(t, nil, got)

	// deployment party domain ids is empty, should return nil
	kds = makeMockDeploymentSummary("cross-domain", "kd-1")
	got = c.updateDeployment(ctx, kds)
	assert.Equal(t, nil, got)

	// deployment is updated, should return nil
	kds = makeMockDeploymentSummary("cross-domain", "kd-1")
	kds.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	kds.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
		"bob": {
			"kd-1": {
				Phase:             "Running",
				Replicas:          1,
				AvailableReplicas: 1,
			},
		},
	}
	got = c.updateDeployment(ctx, kds)
	assert.Equal(t, nil, got)
}
