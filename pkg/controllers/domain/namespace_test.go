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
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func makeTestNamespace(name string) *apicorev1.Namespace {
	return &apicorev1.Namespace{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.LabelDomainName: name,
			},
		},
	}
}

func makeTestNamespaceWithDeletedLabel(name string) *apicorev1.Namespace {
	return &apicorev1.Namespace{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.LabelDomainName:    name,
				common.LabelDomainDeleted: "true",
			},
		},
	}
}

func TestCreateNamespace(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()

	ns1 := makeTestNamespace("ns1")
	kubeFakeClient := clientsetfake.NewSimpleClientset(ns1)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()

	nsInformer.Informer().GetStore().Add(ns1)

	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	cc := c.(*Controller)
	cc.namespaceLister = nsInformer.Lister()

	testCases := []struct {
		name    string
		domain  *kusciaapisv1alpha1.Domain
		wantErr bool
	}{
		{
			name:    "namespace already exist",
			domain:  makeTestDomain("ns1"),
			wantErr: false,
		},
		{
			name:    "namespace does not exist",
			domain:  makeTestDomain("ns2"),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.createNamespace(tc.domain)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestUpdateNamespace(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()

	ns1 := makeTestNamespace("ns1")
	ns2 := makeTestNamespaceWithDeletedLabel("ns2")

	kubeFakeClient := clientsetfake.NewSimpleClientset(ns1, ns2)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()

	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns2)

	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	cc := c.(*Controller)
	cc.namespaceLister = nsInformer.Lister()

	testCases := []struct {
		name    string
		domain  *kusciaapisv1alpha1.Domain
		wantErr bool
	}{
		{
			name:    "namespace is not found",
			domain:  makeTestDomain("ns3"),
			wantErr: true,
		},
		{
			name:    "namespace is not updated",
			domain:  makeTestDomain("ns1"),
			wantErr: false,
		},
		{
			name:    "namespace is updated",
			domain:  makeTestDomain("ns2"),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.updateNamespace(tc.domain)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestDeleteNamespace(t *testing.T) {
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()

	ns1 := makeTestNamespace("ns1")
	ns2 := makeTestNamespaceWithDeletedLabel("ns2")
	ns3 := makeTestNamespace("ns3")

	kubeFakeClient := clientsetfake.NewSimpleClientset(ns1, ns2, ns3)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := kubeInformerFactory.Core().V1().Namespaces()

	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns2)
	nsInformer.Informer().GetStore().Add(ns3)

	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	cc := c.(*Controller)
	cc.namespaceLister = nsInformer.Lister()

	testCases := []struct {
		name       string
		domainName string
		wantErr    bool
	}{
		{
			name:       "namespace is not found",
			domainName: "ns3",
			wantErr:    false,
		},
		{
			name:       "namespace delete label is exist",
			domainName: "ns2",
			wantErr:    false,
		},
		{
			name:       "namespace is updated",
			domainName: "ns3",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.deleteNamespace(tc.domainName)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
