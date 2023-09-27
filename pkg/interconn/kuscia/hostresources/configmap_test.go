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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedOrDeletedConfigMap(t *testing.T) {
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

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "cm",
		},
	}

	c.handleAddedOrDeletedConfigMap(cm)
	if c.configMapsQueue.Len() != 1 {
		t.Error("configmap queue length should be 1")
	}
}

func TestHandleUpdatedConfigMap(t *testing.T) {
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

	cm1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "cm1",
			ResourceVersion: "1",
		},
	}

	cm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "cm1",
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
			oldObj: "cm1",
			newObj: "cm2",
			want:   0,
		},
		{
			name:   "configmap is same",
			oldObj: cm1,
			newObj: cm1,
			want:   0,
		},
		{
			name:   "configmap is updated",
			oldObj: cm1,
			newObj: cm2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedConfigMap(tt.oldObj, tt.newObj)
			if c.configMapsQueue.Len() != tt.want {
				t.Errorf("get %v, want %v", c.configMapsQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncConfigMapHandler(t *testing.T) {
	ctx := context.Background()
	cm1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "cm1",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	cm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "cm2",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	cm3 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "cm3",
			ResourceVersion: "10",
			Labels: map[string]string{
				common.LabelInitiator: "ns2",
			},
		},
	}

	hostKubeFakeClient := clientsetfake.NewSimpleClientset(cm1, cm2, cm3)
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}
	mCm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "cm2",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "10",
			},
		},
	}

	mCm3 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "cm3",
			Labels: map[string]string{
				common.LabelInitiator:                       "ns2",
				common.LabelResourceVersionUnderHostCluster: "1",
			},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset(mCm3)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	cmInformer.Informer().GetStore().Add(mCm2)
	cmInformer.Informer().GetStore().Add(mCm3)

	opt := &Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
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
			name:    "invalid configmap key",
			key:     "ns1/cm/test",
			wantErr: false,
		},
		{
			name:    "configmap namespace is not equal to member",
			key:     "ns3/cm1",
			wantErr: false,
		},
		{
			name:    "configmap is not exist in host cluster",
			key:     "ns1/cm11",
			wantErr: false,
		},
		{
			name:    "configmap is not exist in member cluster",
			key:     "ns1/cm1",
			wantErr: false,
		},
		{
			name:    "configmap label resource version is greater than configmap resource version in host cluster",
			key:     "ns1/cm2",
			wantErr: false,
		},
		{
			name:    "configmap label resource version is less than configmap resource version in host cluster",
			key:     "ns1/cm3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncConfigMapHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}
