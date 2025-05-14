/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/tests/util"
)

func TestNewTaskResourceManager(t *testing.T) {
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()

	timeout := 10 * time.Second
	trMgr := NewTaskResourceManager(cs, nil, trInformer, podInformer, nsInformer, &timeout)
	if trMgr == nil {
		t.Error("expected not nil, got nil")
	}
}

func TestPreFilter(t *testing.T) {
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.UID = "111"
	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.UID = "222"
	tr3 := util.MakeTaskResource("ns1", "tr3", 1, nil)
	tr3.UID = "333"
	tr1.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserving
	tr2.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseFailed
	tr3.Status.Phase = kusciaapisv1alpha1.TaskResourcePhaseReserving

	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)
	trInformer.Informer().GetStore().Add(tr3)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
		},
	}
	nsInformer.Informer().GetStore().Add(ns1)

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "333"}, nil, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)

	tests := []struct {
		name        string
		pod         *corev1.Pod
		cleanTr     *kusciaapisv1alpha1.TaskResource
		expectedErr bool
	}{
		{
			name:        "pod does not belong to any task resource",
			pod:         st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj(),
			expectedErr: false,
		},
		{
			name: "pod belong to task resource tr1 but tr1 does node exist",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			cleanTr:     tr1,
			expectedErr: true,
		},
		{
			name: "pod belong to task resource tr2 but task resource phase is failed",
			pod: st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr2").Label(kusciaapisv1alpha1.TaskResourceUID, "222").Obj(),
			expectedErr: true,
		},
		{
			name: "pod belong to task resource tr1 but the member less than MinReservedPods",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			expectedErr: true,
		},
		{
			name: "pod belong to task resource tr3 and the reserved pods number greater than MinReservedPods",
			pod: st.MakePod().Name("pod3").Namespace("ns1").Node("node3").UID("pod3").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr3").Label(kusciaapisv1alpha1.TaskResourceUID, "333").Obj(),
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		timeout := 10 * time.Second
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanTr != nil {
				trInformer.Informer().GetStore().Delete(tt.cleanTr)
			}

			defer func() {
				if tt.cleanTr != nil {
					trInformer.Informer().GetStore().Add(tt.cleanTr)
				}
			}()

			podInformer.Informer().GetStore().Add(tt.pod)

			trMgr := NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			err := trMgr.PreFilter(context.Background(), tt.pod)
			if err != nil != tt.expectedErr {
				t.Errorf("expectedErr %v, got %v", tt.expectedErr, err != nil)
			}
		})
	}
}

func TestReserve(t *testing.T) {
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	tr1 := util.MakeTaskResource("ns1", "tr1", 2, nil)
	tr1.UID = "111"
	trInformer.Informer().GetStore().Add(tr1)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()

	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"}, nil, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)

	type settr struct {
		trInfo []taskResourceInfo
		trName string
	}

	tests := []struct {
		name            string
		pod             *corev1.Pod
		settr           settr
		expectedtrInfos []string
	}{
		{
			name:            "pod does not belong to any task resource",
			pod:             st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj(),
			expectedtrInfos: nil,
		},
		{
			name: "pod belong to task resource tr1",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			expectedtrInfos: []string{"ns1/tr1"},
		},
		{
			name: "pod belong to task resource tr1 and tr1 already is stored in taskResourceInfos",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			settr: settr{
				trInfo: []taskResourceInfo{
					{
						nodeName: "node1",
						podName:  "pod1",
					},
				},
				trName: "ns1/tr1",
			},
			expectedtrInfos: []string{"ns1/tr1"},
		},
		{
			name: "pod belong to task resource tr1 and taskResourceInfos store multi elements for tr1",
			pod:  st.MakePod().Name("pod1").Namespace("ns11").Node("node1").UID("pod1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			settr: settr{
				trInfo: []taskResourceInfo{
					{
						nodeName: "node11",
						podName:  "pod11",
					},
				},
				trName: "ns11/tr1",
			},
			expectedtrInfos: []string{"ns11/tr1"},
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trMgr := NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			if tt.settr.trName != "" {
				trMgr.taskResourceInfos.Store(tt.settr.trName, tt.settr.trInfo)
			}

			trMgr.Reserve(context.Background(), tt.pod)
			for _, trName := range tt.expectedtrInfos {
				if _, ok := trMgr.taskResourceInfos.Load(trName); !ok {
					t.Errorf("expected tr %s, but not found", trName)
				}
			}
		})
	}
}

func TestPermit(t *testing.T) {
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	tr1 := util.MakeTaskResource("ns1", "tr1", 1, nil)
	tr1.UID = "111"
	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.UID = "222"
	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"}, map[string]string{kusciaapisv1alpha1.TaskResourceKey: "tr1"}, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)

	existingPods, allNodes = util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "222"}, map[string]string{kusciaapisv1alpha1.TaskResourceKey: "tr2"}, 60, 30)
	groupSnapshot := util.NewFakeSharedLister(existingPods, allNodes)

	tests := []struct {
		name          string
		pod           *corev1.Pod
		groupSnapshot framework.SharedLister
		expected      Status
	}{
		{
			name:     "pods do not belong to any task resource, Not Specified",
			pod:      st.MakePod().Name("pod1").UID("pod1").Obj(),
			expected: TaskResourceNotSpecified,
		},
		{
			name: "pods belong to a tr3, Not Found",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr3").Label(kusciaapisv1alpha1.TaskResourceUID, "333").Obj(),
			expected: TaskResourceNotFound,
		},
		{
			name: "pods belong to tr1, Allow",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr1").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			groupSnapshot: groupSnapshot,
			expected:      Success,
		},
		{
			name: "pods belong to tr1, Wait",
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr2").Label(kusciaapisv1alpha1.TaskResourceUID, "222").Obj(),
			expected: Wait,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgrSnapshot := snapshot
			if tt.groupSnapshot != nil {
				mgrSnapshot = tt.groupSnapshot
			}
			trMgr := NewTaskResourceManager(cs, mgrSnapshot, trInformer, podInformer, nsInformer, &timeout)
			trMgr.Reserve(context.Background(), tt.pod)
			got := trMgr.Permit(context.Background(), tt.pod)
			if got != tt.expected {
				t.Errorf("expected %v, got: %v", tt.expected, got)
			}
		})
	}
}

func TestDeletePermittedTaskResource(t *testing.T) {
	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	trInformer.Informer().GetStore().Add(tr)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()

	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"}, nil, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)

	tests := []struct {
		name             string
		tr               *kusciaapisv1alpha1.TaskResource
		pods             []*corev1.Pod
		isPatchingtrList []string
		expectedtrExist  bool
	}{
		{
			name: "pod1 belong to task resource tr and task resource is patching",
			tr:   tr,
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").Label(kusciaapisv1alpha1.TaskResourceKey, "tr").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").Label(kusciaapisv1alpha1.TaskResourceKey, "tr").Obj()},
			isPatchingtrList: []string{"ns1/tr"},
			expectedtrExist:  false,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trMgr := NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			for _, pod := range tt.pods {
				trMgr.Reserve(context.Background(), pod)
			}

			for _, trName := range tt.isPatchingtrList {
				trMgr.patchTaskResourceInfos.Store(trName, true)
			}

			trMgr.DeletePermittedTaskResource(tr)
			trInfo := GetTaskResourceInfos(trMgr)
			_, exist := trInfo.Load("ns1/tr")
			if exist != tt.expectedtrExist {
				t.Errorf("expectedtrExist %v, got %v", tt.expectedtrExist, exist)
			}
		})
	}
}

func TestCalculateAssignedPods(t *testing.T) {
	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.UID = "111"
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	trInformer.Informer().GetStore().Add(tr)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()

	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"},
		map[string]string{kusciaapisv1alpha1.TaskResourceKey: "tr"}, 60, 30)
	for _, pod := range existingPods {
		pod.Namespace = "ns1"
	}
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)

	tests := []struct {
		name         string
		tr           *kusciaapisv1alpha1.TaskResource
		pod          *corev1.Pod
		wrongtrInfos bool
		trInfos      []taskResourceInfo
		expected     int
	}{
		{
			name: "pod1 belong to tr but tr does not exist in taskResourceInfos",
			tr:   tr,
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			expected: 0,
		},
		{
			name: "pod1 belong to tr but trInfos type is invalid",
			tr:   tr,
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			wrongtrInfos: true,
			trInfos: []taskResourceInfo{
				{
					nodeName: "node-2",
					podName:  "pod2",
				},
			},
			expected: 0,
		},
		{
			name: "pod1 belong to tr but node info does not exist",
			tr:   tr,
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node-1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			trInfos: []taskResourceInfo{
				{
					nodeName: "node-2",
					podName:  "pod2",
				},
			},
			expected: 0,
		},
		{
			name: "pod1 belong to tr but node name does not exist",
			tr:   tr,
			pod: st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
				Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
			trInfos: []taskResourceInfo{
				{
					podName: "pod1",
				},
				{
					nodeName: "node1",
					podName:  "pod31",
				},
			},
			expected: 1,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trMgr := NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			if tt.wrongtrInfos && len(tt.trInfos) > 0 {
				trMgr.taskResourceInfos.Store(tt.tr.Namespace+"/"+tt.tr.Name, tt.trInfos[0])
			} else if len(tt.trInfos) > 0 {
				trMgr.taskResourceInfos.Store(tt.tr.Namespace+"/"+tt.tr.Name, tt.trInfos)
			}

			got := trMgr.CalculateAssignedPods(tt.tr, tt.pod)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGetWaitTimeDuration(t *testing.T) {
	rt := 10
	tests := []struct {
		name                    string
		tr                      *kusciaapisv1alpha1.TaskResource
		resourceReservedSeconds time.Duration
		expected                time.Duration
	}{
		{
			name: "tr is not nil",
			tr: &kusciaapisv1alpha1.TaskResource{
				Spec: kusciaapisv1alpha1.TaskResourceSpec{
					ResourceReservedSeconds: rt,
				},
			},
			resourceReservedSeconds: 20,
			expected:                10 * time.Second,
		},
		{
			name:                    "tr is nil and resourceReservedSeconds is not nil",
			resourceReservedSeconds: 20,
			expected:                20 * time.Second,
		},
		{
			name:     "tr and resourceReservedSeconds are nil",
			expected: defaultWaitTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetWaitTimeDuration(tt.tr, &tt.resourceReservedSeconds)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGetScheduledPodsCount(t *testing.T) {
	pod1 := st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").Label(kusciaapisv1alpha1.TaskResourceKey, "tr").Obj()
	pod2 := st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").Label(kusciaapisv1alpha1.TaskResourceKey, "tr").Obj()
	tests := []struct {
		name     string
		trInfos  []taskResourceInfo
		pods     []*corev1.Pod
		expected []*corev1.Pod
	}{
		{
			name: "pods is empty",
			trInfos: []taskResourceInfo{
				{
					podName:  "pod3",
					nodeName: "node3",
				},
			},
			expected: nil,
		},
		{
			name: "pods include pod1 and pod2",
			trInfos: []taskResourceInfo{
				{
					podName:  "pod3",
					nodeName: "node3",
				},
				{
					podName:  "pod1",
					nodeName: "node1",
				},
			},
			pods:     []*corev1.Pod{pod1, pod2},
			expected: []*corev1.Pod{pod2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getScheduledPodsCount(tt.trInfos, tt.pods)
			if got != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
