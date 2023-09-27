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
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedPod(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	pod := st.MakePod().Name("pod").Namespace("test-1").Obj()
	cc := c.(*Controller)
	cc.handleAddedPod(pod)
	if cc.podQueue.Len() != 1 {
		t.Error("pod queue length should be 1")
	}
}

func TestHandleUpdatedPod(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	pod1 := st.MakePod().Name("pod1").Namespace("test-1").Obj()
	pod2 := st.MakePod().Name("pod1").Namespace("test-1").Label(common.LabelInitiator, "task-1").Obj()
	pod1.ResourceVersion = "1"
	pod2.ResourceVersion = "2"

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
			name:   "pod is same",
			oldObj: pod1,
			newObj: pod1,
			want:   0,
		},
		{
			name:   "pod is updated",
			oldObj: pod1,
			newObj: pod2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedPod(tt.oldObj, tt.newObj)
			if cc.podQueue.Len() != tt.want {
				t.Errorf("got %v, want %v", cc.podQueue.Len(), tt.want)
			}
		})
	}
}

func TestSyncPodHandler(t *testing.T) {
	ctx := context.Background()

	hostPod2 := st.MakePod().Namespace("ns1").Name("pod2").Labels(map[string]string{
		common.LabelInitiator: "ns2",
	}).Obj()
	hostPod2.ResourceVersion = "10"

	hostPod3 := st.MakePod().Namespace("ns1").Name("pod3").Labels(map[string]string{
		common.LabelInitiator: "ns2",
	}).Obj()
	hostPod3.ResourceVersion = "1"

	hostKubeFakeClient := clientsetfake.NewSimpleClientset(hostPod2, hostPod3)
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
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

	pod1 := st.MakePod().Namespace("ns1").Name("pod1").Labels(map[string]string{
		common.LabelResourceVersionUnderHostCluster: "2",
		common.LabelInitiator:                       "ns2",
	}).Obj()

	pod2 := st.MakePod().Namespace("ns1").Name("pod2").Labels(map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns2",
	}).Obj()

	pod3 := st.MakePod().Namespace("ns1").Name("pod3").Labels(map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns2",
	}).Obj()

	pod4 := st.MakePod().Namespace("ns1").Name("pod4").Labels(map[string]string{
		common.LabelResourceVersionUnderHostCluster: "3",
		common.LabelInitiator:                       "ns3",
	}).Obj()

	podInformer.Informer().GetStore().Add(pod1)
	podInformer.Informer().GetStore().Add(pod2)
	podInformer.Informer().GetStore().Add(pod3)
	podInformer.Informer().GetStore().Add(pod4)

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
		podLister:           podInformer.Lister(),
		hostResourceManager: hrm,
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid pod key",
			key:     "ns1/pod1/test",
			wantErr: false,
		},
		{
			name:    "pod is not exist in member cluster",
			key:     "ns1/pod5",
			wantErr: false,
		},
		{
			name:    "pod host resource accessor is empty",
			key:     "ns1/pod4",
			wantErr: false,
		},
		{
			name:    "pod is not exit in host cluster",
			key:     "ns1/pod1",
			wantErr: false,
		},
		{
			name:    "pod label resource version is less than pod resource version in host cluster",
			key:     "ns1/pod2",
			wantErr: true,
		},
		{
			name:    "patch pod successfully",
			key:     "ns1/pod3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.syncPodHandler(ctx, tt.key)
			if got != nil != tt.wantErr {
				t.Errorf("sync pod failed, got %v, want %v", got, tt.wantErr)
			}
		})
	}
}
