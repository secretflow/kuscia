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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/test/util"
)

func TestNewController(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
}

func TestResourceFilter(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(context.Background().Done())

	ns1 := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
	ns2 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ns2",
			Labels: map[string]string{common.LabelDomainRole: string(kusciaapisv1alpha1.Partner)},
		}}

	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns2)

	tr1 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr1.Labels = map[string]string{
		common.LabelResourceVersionUnderHostCluster: "1",
		common.LabelInitiator:                       "ns2",
		common.LabelInterConnProtocolType:           string(kusciaapisv1alpha1.InterConnKuscia),
	}

	c := &Controller{
		namespaceLister: nsInformer.Lister(),
	}

	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "invalid obj type",
			obj:  "test-obj",
			want: false,
		},
		{
			name: "obj is pod but label does not match",
			obj:  st.MakePod().Name("pod1").Namespace("ns1").Obj(),
			want: false,
		},
		{
			name: "obj is pod and namespace label domain role is partner",
			obj:  st.MakePod().Name("pod1").Namespace("ns2").Obj(),
			want: false,
		},
		{
			name: "obj is pod and pod label is empty",
			obj:  st.MakePod().Name("pod1").Namespace("ns1").Obj(),
			want: false,
		},
		{
			name: "obj is pod and pod belong to initiator",
			obj:  st.MakePod().Name("pod1").Namespace("ns1").Label(common.LabelInitiator, "ns1").Obj(),
			want: false,
		},
		{
			name: "obj is pod and pod label is valid",
			obj: st.MakePod().Name("pod1").Namespace("ns1").Labels(map[string]string{
				common.LabelResourceVersionUnderHostCluster: "1",
				common.LabelInitiator:                       "ns2",
				common.LabelInterConnProtocolType:           string(kusciaapisv1alpha1.InterConnKuscia),
			}).Obj(),
			want: true,
		},
		{
			name: "obj is task resource and task resource label is valid",
			obj:  tr1,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.resourceFilter(tt.obj); got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestStop(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	cc := c.(*Controller)
	cc.Stop()
	if cc.cancel != nil {
		t.Errorf("controller cancel should be nil")
	}
}

func TestCleanupResidualResources(t *testing.T) {
	pod1 := st.MakePod().Namespace("ns1").Name("pod1").Obj()
	pod2 := st.MakePod().Namespace("ns1").Name("pod2").Labels(map[string]string{common.LabelInitiator: "test", common.LabelResourceVersionUnderHostCluster: "1"}).Obj()

	service1 := &v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "svc1"}}
	service2 := &v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "svc2", Labels: map[string]string{common.LabelInitiator: "test", common.LabelResourceVersionUnderHostCluster: "1"}}}

	cm1 := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "cm1"}}
	cm2 := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "cm2", Labels: map[string]string{common.LabelInitiator: "test", common.LabelResourceVersionUnderHostCluster: "1"}}}

	tr1 := &kusciaapisv1alpha1.TaskResource{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "tr1"}}
	tr2 := &kusciaapisv1alpha1.TaskResource{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "tr2", Labels: map[string]string{common.LabelInitiator: "test", common.LabelResourceVersionUnderHostCluster: "1"}}}

	kubeFakeClient := clientsetfake.NewSimpleClientset(pod1, pod2, service1, service2, cm1, cm2)
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset(tr1, tr2)
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	podInformer := informerFactory.Core().V1().Pods()
	svcInformer := informerFactory.Core().V1().Services()
	cmInformer := informerFactory.Core().V1().ConfigMaps()

	podInformer.Informer().GetStore().Add(pod1)
	podInformer.Informer().GetStore().Add(pod2)

	svcInformer.Informer().GetStore().Add(service1)
	svcInformer.Informer().GetStore().Add(service2)

	cmInformer.Informer().GetStore().Add(cm1)
	cmInformer.Informer().GetStore().Add(cm2)

	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)

	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)
	cc.interopConfigInfos = map[string]*interopConfigInfo{
		"ic": {
			host:    "ns1",
			members: []string{"ns2"},
		},
	}
	cc.podLister = podInformer.Lister()
	cc.serviceLister = svcInformer.Lister()
	cc.configMapLister = cmInformer.Lister()
	cc.taskResourceLister = trInformer.Lister()

	cc.cleanupResidualResources()
}

func TestName(t *testing.T) {
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c.Name() != controllerName {
		t.Errorf("name func should return %v", controllerName)
	}
}
