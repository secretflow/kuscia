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

//nolint:dulp
package hostresources

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedOrDeletedDomainDataGrant(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	opt := &hostResourcesControllerOptions{
		host:               "ns1",
		member:             "ns2",
		memberKubeClient:   kubeFakeClient,
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	ddg := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "dd",
		},
	}

	c.handleAddedOrDeletedDomainDataGrant(ddg)
	if c.domainDataGrantQueue.Len() != 1 {
		t.Error("dd queue length should be 1")
	}
}

func TestHandleUpdatedDomainDataGrant(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	opt := &hostResourcesControllerOptions{
		host:               "ns1",
		member:             "ns2",
		memberKubeClient:   kubeFakeClient,
		memberKusciaClient: kusciaFakeClient,
	}

	c, err := newHostResourcesController(opt)
	if err != nil {
		t.Error("new controller failed")
	}

	ddg1 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "ddg1",
			ResourceVersion: "1",
		},
	}

	ddg2 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "ddg2",
			ResourceVersion: "2",
		},
	}

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "object type is invalid",
			oldObj: "pod1",
			newObj: "pod2",
			want:   0,
		},
		{
			name:   "domaindatagrant is same",
			oldObj: ddg1,
			newObj: ddg1,
			want:   0,
		},
		{
			name:   "domaindatagrant is updated",
			oldObj: ddg1,
			newObj: ddg2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedDomainDataGrant(tt.oldObj, tt.newObj)
			if c.domainDataGrantQueue.Len() != tt.want {
				t.Errorf("get %v, want %v", c.domainDataGrantQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncDomainDataGrantHandler(t *testing.T) {
	ctx := context.Background()
	ddg1 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "ddg1",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInterConnProtocolType: "kuscia",
				common.LabelInitiator:             "ns2",
			},
		},
	}

	ddg2 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "ddg2",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInterConnProtocolType: "kuscia",
				common.LabelInitiator:             "ns2",
			},
		},
	}

	ddg3 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "ddg3",
			ResourceVersion: "10",
			Labels: map[string]string{
				common.LabelInterConnProtocolType: "kuscia",
				common.LabelInitiator:             "ns2",
			},
		},
	}

	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(ddg1, ddg2, ddg3)
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}
	mDdg2 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "ddg2",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "10",
			},
		},
	}

	mDdg3 := &kusciaapisv1alpha1.DomainDataGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "ddg3",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "1",
			},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	//	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(mDdg3)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)

	domaindataGrantInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDataGrants()

	domaindataGrantInformer.Informer().GetStore().Add(mDdg2)
	domaindataGrantInformer.Informer().GetStore().Add(mDdg3)

	opt := &Options{
		MemberKubeClient:            kubeFakeClient,
		MemberKusciaClient:          kusciaFakeClient,
		MemberDomainDataGrantLister: domaindataGrantInformer.Lister(),
	}
	hrm := NewHostResourcesManager(opt)
	hrm.Register("ns2", "ns1")
	accessor := hrm.GetHostResourceAccessor("ns2", "ns1")
	c := accessor.(*hostResourcesController)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid domaindatagrant key",
			key:     "ns1/domaindatagrant/test",
			wantErr: false,
		},
		{
			name:    "domaindatagrant namespace is not equal to member",
			key:     "ns3/ddg1",
			wantErr: false,
		},
		{
			name:    "domaindatagrant is not exist in host cluster",
			key:     "ns1/ddg11",
			wantErr: false,
		},
		{
			name:    "domaindatagrant is not exist in member cluster",
			key:     "ns1/ddg1",
			wantErr: false,
		},
		{
			name:    "domaindatagrant label resource version is greater than domaindatagrant resource version in host cluster",
			key:     "ns1/ddg2",
			wantErr: false,
		},
		{
			name:    "domaindatagrant label resource version is less than domaindatagrant resource version in host cluster",
			key:     "ns1/ddg3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncDomainDataGrantHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}
