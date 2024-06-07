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
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	kd1 := makeMockDeployment("bob", "kd-1")
	kd2 := makeMockDeployment("bob", "kd-1")
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
			c.handleUpdatedDeployment(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.deploymentQueue.Len())
		})
	}
}

func TestHandleDeletedDeployment(t *testing.T) {
	t.Parallel()
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// enqueue deleted kd, deployment queue length should be equal to 1
	kd := makeMockDeployment("bob", "kd-1")
	c.handleDeletedDeployment(kd)
	assert.Equal(t, 1, c.deploymentQueue.Len())
}

func TestDeleteDeployment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	opt := &hostResourcesControllerOptions{
		host:                   "alice",
		member:                 "bob",
		memberKusciaClient:     kusciaFakeClient,
		memberDeploymentLister: kdInformer.Lister(),
	}

	kd1 := makeMockDeployment("bob", "kd-1")
	kd2 := makeMockDeployment("bob", "kd-2")
	kd2.Annotations[common.InitiatorAnnotationKey] = "alice"
	kdInformer.Informer().GetStore().Add(kd1)
	kdInformer.Informer().GetStore().Add(kd2)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// kd1 initiator label is empty, should return nil
	got := c.deleteDeployment(ctx, "bob", "kd-1")
	assert.Equal(t, nil, got)

	// delete deployment, should return nil
	got = c.deleteDeployment(ctx, "bob", "kd-2")
	assert.Equal(t, nil, got)
}

func TestProcessDeployment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kd1 := makeMockDeployment("cross-domain", "kd-1")
	kusciaFakeClient := fake.NewSimpleClientset(kd1)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainAlice := &v1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{
			Name: "alice",
		},
	}
	domainInformer.Informer().GetStore().Add(domainAlice)
	opt := &hostResourcesControllerOptions{
		host:                   "alice",
		member:                 "bob",
		memberKusciaClient:     kusciaFakeClient,
		memberDomainLister:     domainInformer.Lister(),
		memberDeploymentLister: kdInformer.Lister(),
	}

	kdInformer.Informer().GetStore().Add(kd1)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// create kd, should return nil
	kd := makeMockDeployment("bob", "kd-2")
	got := c.processDeployment(ctx, kd)
	assert.Equal(t, nil, got)

	// update kd, should return nil
	kd = makeMockDeployment("bob", "kd-1")
	kd.Spec.Initiator = "alice"
	got = c.processDeployment(ctx, kd)
	assert.Equal(t, nil, got)
}
