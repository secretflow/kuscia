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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedOrDeletedTaskResource(t *testing.T) {
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

	tr := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "tr",
		},
	}

	c.handleAddedOrDeletedTaskResource(tr)
	if c.taskResourcesQueue.Len() != 1 {
		t.Error("tsak resource queue length should be 1")
	}
}

func TestHandleUpdatedTaskResource(t *testing.T) {
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

	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "tr1",
			ResourceVersion: "1",
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "tr2",
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
			oldObj: "tr1",
			newObj: "tr2",
			want:   0,
		},
		{
			name:   "task resource is same",
			oldObj: tr1,
			newObj: tr1,
			want:   0,
		},
		{
			name:   "task resource is updated",
			oldObj: tr1,
			newObj: tr2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.handleUpdatedTaskResource(tt.oldObj, tt.newObj)
			if c.taskResourcesQueue.Len() != tt.want {
				t.Errorf("get %v, want %v", c.taskResourcesQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncTaskResourceHandler(t *testing.T) {
	ctx := context.Background()
	tr1 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "tr",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelTaskInitiator: "ns2",
			},
		},
	}

	tr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "tr2",
			ResourceVersion: "1",
			Labels: map[string]string{
				common.LabelTaskInitiator: "ns2",
			},
		},
	}

	tr3 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "ns1",
			Name:            "tr3",
			ResourceVersion: "10",
			Labels: map[string]string{
				common.LabelTaskInitiator: "ns2",
			},
		},
	}

	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr1, tr2, tr3)
	GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	mTr2 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "tr2",
			Labels: map[string]string{
				common.LabelTaskInitiator:                   "ns2",
				common.LabelResourceVersionUnderHostCluster: "10",
			},
		},
	}

	mTr3 := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "tr3",
			Labels: map[string]string{
				common.LabelTaskInitiator:                   "ns2",
				common.LabelResourceVersionUnderHostCluster: "1",
			},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(mTr3)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	trInformer.Informer().GetStore().Add(mTr2)
	trInformer.Informer().GetStore().Add(mTr3)

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
			name:    "invalid task resource key",
			key:     "ns1/tr/test",
			wantErr: false,
		},
		{
			name:    "task resource namespace is not equal to member",
			key:     "ns3/tr1",
			wantErr: false,
		},
		{
			name:    "task resource is not exist in host cluster",
			key:     "ns1/tr11",
			wantErr: false,
		},
		{
			name:    "task resource is not exist in member cluster",
			key:     "ns1/tr1",
			wantErr: false,
		},
		{
			name:    "task resource label resource version is greater than task resource resource version in host cluster",
			key:     "ns1/tr2",
			wantErr: false,
		},
		{
			name:    "task resource label resource version is less than task resource resource version in host cluster",
			key:     "ns1/tr3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncTaskResourceHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("got: %v, want: %v", got != nil, tt.wantErr)
			}
		})
	}
}
