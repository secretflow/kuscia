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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func makeMockDomainDataGrant(namespace, name string) *kusciaapisv1alpha1.DomainDataGrant {
	ddg := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	return ddg
}

func TestHandleUpdatedDomainDataGrant(t *testing.T) {
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	ddg1 := makeMockDomainDataGrant("bob", "ddg-1")
	ddg1.ResourceVersion = "1"
	ddg2 := makeMockDomainDataGrant("bob", "ddg-2")
	ddg2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "object type is invalid",
			oldObj: "ddg1",
			newObj: "ddg2",
			want:   0,
		},
		{
			name:   "domain data grant is same",
			oldObj: ddg1,
			newObj: ddg1,
			want:   0,
		},
		{
			name:   "domain data grant is updated",
			oldObj: ddg1,
			newObj: ddg2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedDomainDataGrant(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.domainDataGrantQueue.Len())
		})
	}
}

func TestHandleDeletedDomainDataGrant(t *testing.T) {
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// resource type is not domain data grant, queue length should be equal to 0
	c.handleDeletedDomainDataGrant("ddg-1")
	assert.Equal(t, 0, c.domainDataGrantQueue.Len())

	// party domain ids is empty, queue length should be equal to 0
	ddg := makeMockDomainDataGrant("bob", "kddg-1")
	c.handleDeletedDomainDataGrant(ddg)
	assert.Equal(t, 0, c.domainDataGrantQueue.Len())

	// enqueue deleted domain data, queue length should be equal to 1
	ddg = makeMockDomainDataGrant("bob", "kddg-1")
	ddg.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	c.handleDeletedDomainDataGrant(ddg)
	assert.Equal(t, 1, c.domainDataGrantQueue.Len())
}

func TestDeleteDomainDataGrant(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ddgInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()
	opt := &hostResourcesControllerOptions{
		host:                        "alice",
		member:                      "bob",
		memberKusciaClient:          kusciaFakeClient,
		memberDomainDataGrantLister: ddgInformer.Lister(),
	}

	ddg1 := makeMockDomainDataGrant("bob", "kddg-1")
	ddgInformer.Informer().GetStore().Add(ddg1)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// domain data grant doesn't exist, should return nil
	got := c.deleteDomainDataGrant(ctx, "bob", "kddg")
	assert.Equal(t, nil, got)

	// domain data grant labels is empty, should return nil
	got = c.deleteDomainDataGrant(ctx, "bob", "kddg-1")
	assert.Equal(t, nil, got)

	// delete domain data grant, should return nil
	ddg1.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.deleteDomainDataGrant(ctx, "bob", "kddg-1")
	assert.Equal(t, nil, got)
}

func TestCreateDomainDataGrant(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainAlice := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}
	domainInformer.Informer().GetStore().Add(domainAlice)
	opt := &hostResourcesControllerOptions{
		host:               "alice",
		member:             "bob",
		memberDomainLister: domainInformer.Lister(),
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	ddg := makeMockDomainDataGrant("bob", "kddg-1")
	got := c.createDomainDataGrant(ctx, ddg, "bob")
	assert.Equal(t, nil, got)
}

func TestUpdateDomainDataGrant(t *testing.T) {
	ctx := context.Background()
	ddg := makeMockDomainDataGrant("bob", "kddg")
	kusciaFakeClient := fake.NewSimpleClientset(ddg)
	opt := &hostResourcesControllerOptions{
		host:               "alice",
		member:             "bob",
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// host domain data grant resource version is not greater than member, should return nil
	memberDdg := makeMockDomainDataGrant("bob", "kddg")
	hostDdg := makeMockDomainDataGrant("bob", "kddg")
	got := c.updateDomainDataGrant(ctx, hostDdg, memberDdg)
	assert.Equal(t, nil, got)

	// update member domain data grant, should return nil
	memberDdg = makeMockDomainDataGrant("bob", "kddg")
	hostDdg = makeMockDomainDataGrant("bob", "kddg")
	hostDdg.ResourceVersion = "2"
	got = c.updateDomainDataGrant(ctx, hostDdg, memberDdg)
	assert.Equal(t, nil, got)
}
