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

package kuscia

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func makeTestService(name, namespace, rv, initiator string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
			ResourceVersion: rv,
			Labels: map[string]string{
				common.LabelInitiator: initiator,
			},
		},
	}
}

func TestHandleAddedService(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}

	svc := makeTestService("svc", "ns1", "1", "ns2")

	cc := c.(*Controller)
	cc.handleAddedService(svc)
	if cc.serviceQueue.Len() != 1 {
		t.Error("service queue length should be 1")
	}
}

func TestHandleUpdatedService(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	svc1 := makeTestService("svc", "ns1", "1", "ns2")
	svc2 := makeTestService("svc", "ns1", "2", "ns2")

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "object type is invalid",
			oldObj: "svc1",
			newObj: "svc2",
			want:   0,
		},
		{
			name:   "service is same",
			oldObj: svc1,
			newObj: svc1,
			want:   0,
		},
		{
			name:   "service is updated",
			oldObj: svc1,
			newObj: svc2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedService(tt.oldObj, tt.newObj)
			if cc.serviceQueue.Len() != tt.want {
				t.Errorf("got %v, want %v", cc.serviceQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncServiceHandler(t *testing.T) {
	ctx := context.Background()

	hostSvc2 := makeTestService("svc2", "ns1", "10", "ns2")
	hostSvc3 := makeTestService("svc3", "ns1", "1", "ns2")

	hostKubeFakeClient := clientsetfake.NewSimpleClientset(hostSvc2, hostSvc3)
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	hostresources.GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	svcInformer := kubeInformerFactory.Core().V1().Services()

	svc1 := makeTestService("svc1", "ns1", "1", "ns2")
	svc1.Labels[common.LabelResourceVersionUnderHostCluster] = "2"

	svc2 := makeTestService("svc2", "ns1", "1", "ns2")
	svc2.Labels[common.LabelResourceVersionUnderHostCluster] = "3"

	svc3 := makeTestService("svc3", "ns1", "1", "ns2")
	svc3.Labels[common.LabelResourceVersionUnderHostCluster] = "3"

	svc4 := makeTestService("svc4", "ns1", "1", "ns3")
	svc4.Labels[common.LabelResourceVersionUnderHostCluster] = "3"

	svcInformer.Informer().GetStore().Add(svc1)
	svcInformer.Informer().GetStore().Add(svc2)
	svcInformer.Informer().GetStore().Add(svc3)
	svcInformer.Informer().GetStore().Add(svc4)

	opt := &hostresources.Options{
		MemberKubeClient:    kubeFakeClient,
		MemberServiceLister: svcInformer.Lister(),
	}
	hrm := hostresources.NewHostResourcesManager(opt)
	hrm.Register("ns2", "ns1")
	for !hrm.GetHostResourceAccessor("ns2", "ns1").HasSynced() {
	}

	c := &Controller{
		serviceLister:       svcInformer.Lister(),
		hostResourceManager: hrm,
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid service key",
			key:     "ns1/svc1/test",
			wantErr: false,
		},
		{
			name:    "service is not exist in member cluster",
			key:     "ns1/svc5",
			wantErr: false,
		},
		{
			name:    "service host resource accessor is empty",
			key:     "ns1/svc4",
			wantErr: false,
		},
		{
			name:    "service is not exit in host cluster",
			key:     "ns1/svc1",
			wantErr: false,
		},
		{
			name:    "service label resource version is less than service resource version in host cluster",
			key:     "ns1/svc2",
			wantErr: true,
		},
		{
			name:    "patch service successfully",
			key:     "ns1/svc3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncServiceHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("sync service failed, got %v, want %v", got, tt.wantErr)
			}
		})
	}
}
