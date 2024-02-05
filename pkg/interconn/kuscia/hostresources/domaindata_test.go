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

func makeMockDomainData(namespace, name string) *kusciaapisv1alpha1.DomainData {
	dd := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	return dd
}

func TestHandleUpdatedDomainData(t *testing.T) {
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	dd1 := makeMockDomainData("bob", "dd-1")
	dd1.ResourceVersion = "1"
	dd2 := makeMockDomainData("bob", "dd-2")
	dd2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "object type is invalid",
			oldObj: "dd1",
			newObj: "dd2",
			want:   0,
		},
		{
			name:   "domain data is same",
			oldObj: dd1,
			newObj: dd1,
			want:   0,
		},
		{
			name:   "domain data is updated",
			oldObj: dd1,
			newObj: dd2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedDomainData(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.domainDataQueue.Len())
		})
	}
}

func TestHandleDeletedDomainData(t *testing.T) {
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// resource type is not domain data, queue length should be equal to 0
	c.handleDeletedDomainData("dd-1")
	assert.Equal(t, 0, c.domainDataQueue.Len())

	// party domain ids is empty, queue length should be equal to 0
	dd := makeMockDomainData("bob", "dd-1")
	c.handleDeletedDomainData(dd)
	assert.Equal(t, 0, c.domainDataQueue.Len())

	// enqueue deleted domain data, queue length should be equal to 1
	dd = makeMockDomainData("bob", "dd-1")
	dd.Annotations[common.InterConnKusciaPartyAnnotationKey] = "bob"
	c.handleDeletedDomainData(dd)
	assert.Equal(t, 1, c.domainDataQueue.Len())
}

func TestDeleteDomainData(t *testing.T) {
	ctx := context.Background()
	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ddInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	opt := &hostResourcesControllerOptions{
		host:                   "alice",
		member:                 "bob",
		memberKusciaClient:     kusciaFakeClient,
		memberDomainDataLister: ddInformer.Lister(),
	}

	dd1 := makeMockDomainData("bob", "dd-1")
	ddInformer.Informer().GetStore().Add(dd1)

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// domain data doesn't exist, should return nil
	got := c.deleteDomainData(ctx, "bob", "dd")
	assert.Equal(t, nil, got)

	// domain data labels is empty, should return nil
	got = c.deleteDomainData(ctx, "bob", "dd-1")
	assert.Equal(t, nil, got)

	// delete domain data, should return nil
	dd1.Annotations[common.InitiatorAnnotationKey] = "alice"
	got = c.deleteDomainData(ctx, "bob", "dd-1")
	assert.Equal(t, nil, got)
}

func TestCreateDomainData(t *testing.T) {
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

	dd := makeMockDomainData("bob", "dd-1")
	got := c.createDomainData(ctx, dd, "bob")
	assert.Equal(t, nil, got)
}

func TestUpdateDomainData(t *testing.T) {
	ctx := context.Background()
	dd := makeMockDomainData("bob", "dd")
	kusciaFakeClient := fake.NewSimpleClientset(dd)
	opt := &hostResourcesControllerOptions{
		host:               "alice",
		member:             "bob",
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// host domainData resource version is not greater than member, should return nil
	memberdd := makeMockDomainData("bob", "dd")
	hostdd := makeMockDomainData("bob", "dd")
	got := c.updateDomainData(ctx, hostdd, memberdd)
	assert.Equal(t, nil, got)

	// update member domain data, should return nil
	memberdd = makeMockDomainData("bob", "dd")
	hostdd = makeMockDomainData("bob", "dd")
	hostdd.ResourceVersion = "2"
	got = c.updateDomainData(ctx, hostdd, memberdd)
	assert.Equal(t, nil, got)
}
