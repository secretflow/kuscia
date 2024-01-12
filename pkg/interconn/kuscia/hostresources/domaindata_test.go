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
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedOrDeletedDomainData(t *testing.T) {
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

	dd := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "dd",
		},
	}

	c.handleAddedOrDeletedDomainData(dd)
	if c.domainDataQueue.Len() != 1 {
		t.Error("domaindata queue length should be 1")
	}
}

func TestHandleUpdatedDomainData(t *testing.T) {
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

	dd1 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "dd1",
			ResourceVersion: "1",
		},
	}

	dd2 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "dd2",
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
			oldObj: "dd1",
			newObj: "dd2",
			want:   0,
		},
		{
			name:   "domaindata is same",
			oldObj: dd1,
			newObj: dd1,
			want:   0,
		},
		{
			name:   "domaindata is updated",
			oldObj: dd1,
			newObj: dd2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedDomainData(tt.oldObj, tt.newObj)
			if c.domainDataQueue.Len() != tt.want {
				t.Errorf("get %v, want %v", c.domainDataQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncDomainDataHandler(t *testing.T) {
	ctx := context.Background()
	dd1 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "dd1",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	dd2 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "dd2",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	dd3 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "dd3",
			ResourceVersion: "10",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(dd1, dd2, dd3)
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	mDomainData2 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "dd2",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "10",
			},
		},
	}

	mDomainData3 := &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "dd3",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "1",
			},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(mDomainData3)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	domainDataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	domainDataInformer.Informer().GetStore().Add(mDomainData2)
	domainDataInformer.Informer().GetStore().Add(mDomainData3)

	opt := &Options{
		MemberKubeClient:       kubeFakeClient,
		MemberKusciaClient:     kusciaFakeClient,
		MemberPodLister:        podInformer.Lister(),
		MemberServiceLister:    svcInformer.Lister(),
		MemberConfigMapLister:  cmInformer.Lister(),
		MemberTrLister:         trInformer.Lister(),
		MemberDomainDataLister: domainDataInformer.Lister(),
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
			name:    "invalid domaindata key",
			key:     "ns1/domaindata/test",
			wantErr: false,
		},
		{
			name:    "domaindata namespace is not equal to member",
			key:     "ns3/dd1",
			wantErr: false,
		},
		{
			name:    "domaindata is not exist in host cluster",
			key:     "ns1/dd11",
			wantErr: false,
		},
		{
			name:    "domaindata is not exist in member cluster",
			key:     "ns1/dd1",
			wantErr: false,
		},
		{
			name:    "domaindata label resource version is greater than domaindata resource version in host cluster",
			key:     "ns1/dd2",
			wantErr: false,
		},
		{
			name:    "domaindata label resource version is less than domaindata resource version in host cluster",
			key:     "ns1/dd3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncDomainDataHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}
