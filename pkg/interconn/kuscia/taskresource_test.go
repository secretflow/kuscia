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

	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/test/util"
)

func TestHandleUpdatedTaskResource(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)

	tr1.ResourceVersion = "1"
	tr2.ResourceVersion = "2"

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
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
			cc.handleUpdatedTaskResource(tt.oldObj, tt.newObj)
			if cc.taskResourceQueue.Len() != tt.want {
				t.Errorf("got %v, want %v", cc.taskResourceQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncTaskResourceHandler(t *testing.T) {
	ctx := context.Background()

	hostTr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	hostTr2.Labels = map[string]string{common.LabelInitiator: "ns2"}
	hostTr2.ResourceVersion = "10"

	hostTr3 := util.MakeTaskResource("ns1", "tr3", 2, nil)
	hostTr3.Labels = map[string]string{common.LabelInitiator: "ns2"}
	hostTr3.ResourceVersion = "1"

	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(hostTr2, hostTr3)
	hostresources.GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	podInformer := kubeInformerFactory.Core().V1().Pods()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	cmInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.Labels = map[string]string{
		common.LabelResourceVersionUnderHostCluster: "2",
		common.LabelInitiator:                       "ns2",
	}

	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.Labels = map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns2",
	}

	tr3 := util.MakeTaskResource("ns1", "tr3", 2, nil)
	tr3.Labels = map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns2",
	}

	tr4 := util.MakeTaskResource("ns1", "tr4", 2, nil)
	tr4.Labels = map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns3",
	}

	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)
	trInformer.Informer().GetStore().Add(tr3)
	trInformer.Informer().GetStore().Add(tr4)

	opt := &hostresources.Options{
		MemberKubeClient:      kubeFakeClient,
		MemberKusciaClient:    kusciaFakeClient,
		MemberPodLister:       podInformer.Lister(),
		MemberServiceLister:   svcInformer.Lister(),
		MemberConfigMapLister: cmInformer.Lister(),
		MemberTrLister:        trInformer.Lister(),
	}
	hrm := hostresources.NewHostResourcesManager(opt)
	hrm.Register("ns2", "ns1")
	for !hrm.GetHostResourceAccessor("ns2", "ns1").HasSynced() {
	}

	c := &Controller{
		taskResourceLister:  trInformer.Lister(),
		hostResourceManager: hrm,
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid task resource key",
			key:     "ns1/tr1/test",
			wantErr: false,
		},
		{
			name:    "task resource is not exist in member cluster",
			key:     "ns1/tr5",
			wantErr: false,
		},
		{
			name:    "task resource host resource accessor is empty",
			key:     "ns1/tr4",
			wantErr: false,
		},
		{
			name:    "task resource is not exit in host cluster",
			key:     "ns1/tr1",
			wantErr: false,
		},
		{
			name:    "task resource label resource version is less than task resource resource version in host cluster",
			key:     "ns1/tr2",
			wantErr: true,
		},
		{
			name:    "patch task resource successfully",
			key:     "ns1/tr3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncTaskResourceHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("sync task resource failed, got %v, want %v", got, tt.wantErr)
			}
		})
	}
}
