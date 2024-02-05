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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
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
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	kj1 := makeMockJob("bob", "kj-1")
	kj2 := makeMockJob("bob", "kj-1")
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
			oldObj: "kj1",
			newObj: "kj2",
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
			c.handleUpdatedJob(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.want, c.jobQueue.Len())
		})
	}
}

func TestHandleDeletedJob(t *testing.T) {
	opt := &hostResourcesControllerOptions{
		host:   "alice",
		member: "bob",
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	// enqueue deleted kj, job queue length should be equal to 1
	kj := makeMockJob("bob", "kj-1")
	c.handleDeletedJob(kj)
	assert.Equal(t, 1, c.jobQueue.Len())
}

func TestSyncJobHandler(t *testing.T) {
	ctx := context.Background()
	hostKj1 := makeMockJob("bob", "kj-1")
	hostKj2 := makeMockJob("bob", "kj-2")
	hostKusciaFakeClient := fake.NewSimpleClientset(hostKj1, hostKj2)
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kusciaFakeClient := fake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	kj := makeMockJob("cross-domain", "kj-1")
	kj.Annotations[common.InitiatorAnnotationKey] = "alice"
	kjInformer.Informer().GetStore().Add(kj)

	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	domainAlice := &v1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{
			Name: "alice",
		},
	}
	domainInformer.Informer().GetStore().Add(domainAlice)

	opt := &Options{
		MemberKusciaClient: kusciaFakeClient,
		MemberDomainLister: domainInformer.Lister(),
		MemberJobLister:    kjInformer.Lister(),
	}

	hrm := NewHostResourcesManager(opt)
	hrm.Register("alice", "bob")
	accessor := hrm.GetHostResourceAccessor("alice", "bob")
	c := accessor.(*hostResourcesController)
	for !c.HasSynced() {
	}

	// delete job, should return nil
	key := fmt.Sprintf("%v%v/%v",
		ikcommon.DeleteEventKeyPrefix, common.KusciaCrossDomain, "kj-1")
	got := c.syncJobHandler(ctx, key)
	assert.Equal(t, nil, got)

	// job already exist, skip creating it, should return nil
	key = "bob/kj-1"
	got = c.syncJobHandler(ctx, key)
	assert.Equal(t, nil, got)

	// job doesn't exist, creat it, should return nil
	key = "bob/kj-2"
	got = c.syncJobHandler(ctx, key)
	assert.Equal(t, nil, got)
}
