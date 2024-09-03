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

package domain

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func makeTestResourceQuota(namespace string) *apicorev1.ResourceQuota {
	return &apicorev1.ResourceQuota{
		ObjectMeta: apismetav1.ObjectMeta{
			Namespace: namespace,
			Name:      resourceQuotaName,
			Labels: map[string]string{
				common.LabelDomainName: resourceQuotaName,
			},
		},
	}
}

func TestCreateResourceQuota(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	domain1 := makeTestDomain("ns1")
	domain1.Spec.ResourceQuota = nil
	domain2 := makeTestDomain("ns2")

	testCases := []struct {
		name    string
		domain  *kusciaapisv1alpha1.Domain
		wantErr bool
	}{
		{
			name:    "domain does not include resource quota",
			domain:  domain1,
			wantErr: false,
		},
		{
			name:    "domain include resource quota",
			domain:  domain2,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.createResourceQuota(tc.domain)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestUpdateResourceQuota(t *testing.T) {
	rq4 := makeTestResourceQuota("ns4")

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kubeFakeClient := clientsetfake.NewSimpleClientset(rq4)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	rqInformer := kubeInformerFactory.Core().V1().ResourceQuotas()

	ns1 := makeTestNamespace("ns1")
	ns3 := makeTestNamespace("ns3")
	ns4 := makeTestNamespace("ns4")
	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns3)
	nsInformer.Informer().GetStore().Add(ns4)

	rq3 := makeTestResourceQuota("ns3")
	rqInformer.Informer().GetStore().Add(rq3)
	rqInformer.Informer().GetStore().Add(rq4)

	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)
	cc.namespaceLister = nsInformer.Lister()
	cc.resourceQuotaLister = rqInformer.Lister()

	domain1 := makeTestDomain("ns1")
	domain2 := makeTestDomain("ns2")
	domain3 := makeTestDomain("ns3")
	domain3.Spec.ResourceQuota = nil
	domain4 := makeTestDomain("ns4")

	testCases := []struct {
		name    string
		domain  *kusciaapisv1alpha1.Domain
		wantErr bool
	}{
		{
			name:    "namespace is not found",
			domain:  domain2,
			wantErr: false,
		},
		{
			name:    "resource quota is not found",
			domain:  domain1,
			wantErr: false,
		},
		{
			name:    "delete resource quota",
			domain:  domain3,
			wantErr: false,
		},
		{
			name:    "update resource quota",
			domain:  domain4,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.updateResourceQuota(tc.domain)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestMakeResourceQuota(t *testing.T) {
	count := 1
	testCases := []struct {
		name      string
		c         *Controller
		domain    *kusciaapisv1alpha1.Domain
		wantValue *apicorev1.ResourceQuota
	}{
		{
			name: "resource quota isn't empty",
			c:    &Controller{},
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: apismetav1.ObjectMeta{
					Name: "test",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{
					ResourceQuota: &kusciaapisv1alpha1.DomainResourceQuota{
						PodMaxCount: &count,
					},
				},
			},
			wantValue: &apicorev1.ResourceQuota{
				ObjectMeta: apismetav1.ObjectMeta{
					Name:      resourceQuotaName,
					Namespace: "test",
					Labels: map[string]string{
						common.LabelDomainName: "test",
					},
				},
				Spec: apicorev1.ResourceQuotaSpec{
					Hard: apicorev1.ResourceList{
						apicorev1.ResourcePods: resource.MustParse(strconv.Itoa(count)),
					},
				},
			},
		},
		{
			name: "resource quota is empty",
			c:    &Controller{},
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: apismetav1.ObjectMeta{
					Name: "test",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{},
			},

			wantValue: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rq := tc.c.makeResourceQuota(tc.domain)
			assert.Equal(t, tc.wantValue, rq)
		})
	}
}

func TestBuildResourceList(t *testing.T) {
	count := 1
	testCases := []struct {
		name      string
		c         *Controller
		rq        *kusciaapisv1alpha1.DomainResourceQuota
		wantValue apicorev1.ResourceList
	}{
		{
			name:      "resource quota is empty",
			c:         &Controller{},
			wantValue: apicorev1.ResourceList{},
		},
		{
			name: "resource quota isn't empty",
			c:    &Controller{},
			rq: &kusciaapisv1alpha1.DomainResourceQuota{
				PodMaxCount: &count,
			},
			wantValue: apicorev1.ResourceList{
				apicorev1.ResourcePods: resource.MustParse(strconv.Itoa(count)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rl := tc.c.buildResourceList(tc.rq)
			assert.Equal(t, tc.wantValue, rl)
		})
	}
}

func TestNeedUpdateResourceList(t *testing.T) {
	count := 1
	testCases := []struct {
		name  string
		c     *Controller
		oldRL apicorev1.ResourceList
		newRL apicorev1.ResourceList
		want  bool
	}{
		{
			name: "doesn't need to update with empty resource list",
			c:    &Controller{},
			want: false,
		},
		{
			name: "doesn't need to update with valid resource list",
			c:    &Controller{},
			oldRL: apicorev1.ResourceList{
				apicorev1.ResourcePods: resource.MustParse(strconv.Itoa(count)),
			},
			newRL: apicorev1.ResourceList{
				apicorev1.ResourcePods: resource.MustParse(strconv.Itoa(count)),
			},
			want: false,
		},
		{
			name: "need to update resource list",
			c:    &Controller{},
			newRL: apicorev1.ResourceList{
				apicorev1.ResourcePods: resource.MustParse(strconv.Itoa(count)),
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := tc.c.needUpdateResourceList(tc.oldRL, tc.newRL)
			assert.Equal(t, tc.want, ok)
		})
	}
}
