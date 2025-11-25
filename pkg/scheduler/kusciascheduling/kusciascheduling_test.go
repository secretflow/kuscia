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

package kusciascheduling

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/defaultbinder"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/queuesort"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	stframework "k8s.io/kubernetes/pkg/scheduler/testing/framework"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/scheduler/kusciascheduling/core"
	"github.com/secretflow/kuscia/tests/util"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		kConfig     runtime.Object
		expectedErr bool
	}{
		{
			name:        "obj is empty",
			kConfig:     nil,
			expectedErr: false,
		},
		{
			name:        "obj type is invalid",
			kConfig:     &corev1.Pod{},
			expectedErr: true,
		},
		{
			name: "obj content type is invalid",
			kConfig: &runtime.Unknown{
				TypeMeta:        runtime.TypeMeta{},
				Raw:             []byte(`{"name": "KusciaScheduling", "args": { "resourceReservedSeconds": 10}}`),
				ContentEncoding: "",
				ContentType:     "",
			},
			expectedErr: true,
		},
		{
			name: "obj body is invalid",
			kConfig: &runtime.Unknown{
				TypeMeta:        runtime.TypeMeta{},
				Raw:             []byte(`"name": "KusciaScheduling"`),
				ContentEncoding: "",
				ContentType:     "application/json",
			},
			expectedErr: true,
		},
		{
			name: "obj is valid",
			kConfig: &runtime.Unknown{
				TypeMeta:        runtime.TypeMeta{},
				Raw:             []byte(`{"name": "KusciaScheduling", "args": { "resourceReservedSeconds": 10}}`),
				ContentEncoding: "",
				ContentType:     "application/json",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArgs(tt.kConfig)
			if err != nil != tt.expectedErr {
				t.Errorf("expected %v, got %v", tt.expectedErr, err != nil)
			}
		})
	}
}

func TestEventsToRegister(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{
			name:     "result should contain Add and Update events",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &KusciaScheduling{}
			got, err := cs.EventsToRegister(context.Background())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(got) != tt.expected {
				t.Errorf("expected %v events, got %v", tt.expected, len(got))
			}

			// Verify that we have both Add and Update events in the combined event
			foundAdd := false
			foundUpdate := false
			for _, event := range got {
				if event.Event.ActionType&framework.Add != 0 {
					foundAdd = true
				}
				if event.Event.ActionType&framework.Update != 0 {
					foundUpdate = true
				}
			}

			if !foundAdd {
				t.Error("expected to find Add event in combined event")
			}
			if !foundUpdate {
				t.Error("expected to find Update event in combined event")
			}
		})
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{
			name:     "result is not empty",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &KusciaScheduling{}
			if got := cs.Name(); len(got) > 0 != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
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

	tr1 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr1.UID = "222"
	tr2 := util.MakeTaskResource("ns1", "tr2", 2, nil)
	tr2.UID = "222"
	trInformer.Informer().GetStore().Add(tr1)
	trInformer.Informer().GetStore().Add(tr2)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "222"},
		map[string]string{kusciaapisv1alpha1.TaskResourceKey: "tr2"}, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)
	// Compose a framework handle.
	registeredPlugins := []stframework.RegisterPluginFunc{
		stframework.RegisterQueueSortPlugin(queuesort.Name, queuesort.New),
		stframework.RegisterBindPlugin(defaultbinder.Name, defaultbinder.New),
	}
	f, err := stframework.NewFramework(ctx, registeredPlugins, "",
		frameworkruntime.WithClientSet(fakeClient),
		frameworkruntime.WithEventRecorder(&events.FakeRecorder{}),
		frameworkruntime.WithInformerFactory(informerFactory),
		frameworkruntime.WithSnapshotSharedLister(snapshot),
		frameworkruntime.WithWaitingPods(frameworkruntime.NewWaitingPodsMap()),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		pods     []*corev1.Pod
		expected framework.Code
	}{
		{
			name: "pods do not belong to any taskResource",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").UID("pod1").Obj()},
			expected: framework.Success,
		},
		{
			name: "pods belong to a taskResource, Wait",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr2").Label(kusciaapisv1alpha1.TaskResourceUID, "222").Obj()},
			expected: framework.Wait,
		},
		{
			name: "pods belong to a taskResource, Allow",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr2").Label(kusciaapisv1alpha1.TaskResourceUID, "222").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr2").Label(kusciaapisv1alpha1.TaskResourceUID, "222").Obj()},
			expected: framework.Success,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trMgr := core.NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			centralizedScheduling := &KusciaScheduling{trMgr: trMgr, frameworkHandler: f}
			for _, pod := range tt.pods {
				centralizedScheduling.Reserve(context.Background(), framework.NewCycleState(), pod, pod.Spec.NodeName)
			}
			code, _ := centralizedScheduling.Permit(context.Background(), framework.NewCycleState(), tt.pods[0], tt.pods[0].Spec.NodeName)
			if code.Code() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, code.Code())
			}
		})
	}
}

func TestPostBind(t *testing.T) {
	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.UID = "111"
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset(tr)
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	trInformer.Informer().GetStore().Add(tr)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"}, nil, 10, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)
	// Compose a framework handle.
	registeredPlugins := []stframework.RegisterPluginFunc{
		stframework.RegisterQueueSortPlugin(queuesort.Name, queuesort.New),
		stframework.RegisterBindPlugin(defaultbinder.Name, defaultbinder.New),
	}
	f, err := stframework.NewFramework(ctx, registeredPlugins, "",
		frameworkruntime.WithClientSet(fakeClient),
		frameworkruntime.WithEventRecorder(&events.FakeRecorder{}),
		frameworkruntime.WithInformerFactory(informerFactory),
		frameworkruntime.WithSnapshotSharedLister(snapshot),
		frameworkruntime.WithWaitingPods(frameworkruntime.NewWaitingPodsMap()),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		pods            []*corev1.Pod
		expectedtrExist bool
	}{
		{
			name: "pod1 does not belong to any task resource",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedtrExist: true,
		},
		{
			name: "pod1 belong to task resource tr",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedtrExist: false,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleState := framework.NewCycleState()
			trMgr := core.NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			centralizedScheduling := &KusciaScheduling{trMgr: trMgr, frameworkHandler: f}
			for _, pod := range tt.pods {
				centralizedScheduling.Reserve(context.Background(), cycleState, pod, pod.Spec.NodeName)
			}
			centralizedScheduling.PostBind(context.Background(), cycleState, tt.pods[0], tt.pods[0].Spec.NodeName)
			trInfo := core.GetTaskResourceInfos(trMgr)
			_, exist := trInfo.Load(tr.Namespace + "/" + tr.Name)
			if exist != tt.expectedtrExist {
				t.Errorf("expectedtrExist %v, got %v", tt.expectedtrExist, exist)
			}
		})
	}
}

func TestPostFilter(t *testing.T) {
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.UID = "111"
	trInformer.Informer().GetStore().Add(tr)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{"test": "a"}, nil, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)
	// Compose a framework handle.
	registeredPlugins := []stframework.RegisterPluginFunc{
		stframework.RegisterQueueSortPlugin(queuesort.Name, queuesort.New),
		stframework.RegisterBindPlugin(defaultbinder.Name, defaultbinder.New),
	}
	f, err := stframework.NewFramework(ctx, registeredPlugins, "",
		frameworkruntime.WithClientSet(fakeClient),
		frameworkruntime.WithEventRecorder(&events.FakeRecorder{}),
		frameworkruntime.WithInformerFactory(informerFactory),
		frameworkruntime.WithSnapshotSharedLister(snapshot),
		frameworkruntime.WithWaitingPods(frameworkruntime.NewWaitingPodsMap()),
	)
	if err != nil {
		t.Fatal(err)
	}

	existingPods, allNodes = util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"},
		map[string]string{kusciaapisv1alpha1.TaskResourceKey: "tr"}, 10, 30)
	for _, pod := range existingPods {
		pod.Namespace = "ns1"
	}
	groupPodSnapshot := util.NewFakeSharedLister(existingPods, allNodes)

	tests := []struct {
		name                 string
		pods                 []*corev1.Pod
		expectedEmptyMsg     bool
		snapshotSharedLister framework.SharedLister
	}{
		{
			name:             "pod does not belong to any task resource",
			pods:             []*corev1.Pod{st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj()},
			expectedEmptyMsg: false,
		},
		{
			name: "enough pods assigned, do not reject all",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
				st.MakePod().Name("pod3").Namespace("ns1").Node("node3").UID("pod3").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedEmptyMsg:     true,
			snapshotSharedLister: groupPodSnapshot,
		},
		{
			name: "pod failed at filter phase, reject all pods",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedEmptyMsg: false,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleState := framework.NewCycleState()
			mgrSnapShot := snapshot
			if tt.snapshotSharedLister != nil {
				mgrSnapShot = tt.snapshotSharedLister
			}

			trMgr := core.NewTaskResourceManager(cs, mgrSnapShot, trInformer, podInformer, nsInformer, &timeout)
			centralizedScheduling := &KusciaScheduling{trMgr: trMgr, frameworkHandler: f}
			for _, pod := range tt.pods {
				centralizedScheduling.Reserve(context.Background(), cycleState, pod, pod.Spec.NodeName)
			}
			_, code := centralizedScheduling.PostFilter(context.Background(), cycleState, tt.pods[0], nil)
			if code.Message() == "" != tt.expectedEmptyMsg {
				t.Errorf("expectedEmptyMsg %v, got %v", tt.expectedEmptyMsg, code.Message() == "")
			}
		})
	}
}

func TestPreFilter(t *testing.T) {
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset()
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.UID = "111"
	trInformer.Informer().GetStore().Add(tr)

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

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{"test": "a"}, nil, 60, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)
	// Compose a framework handle.
	registeredPlugins := []stframework.RegisterPluginFunc{
		stframework.RegisterQueueSortPlugin(queuesort.Name, queuesort.New),
		stframework.RegisterBindPlugin(defaultbinder.Name, defaultbinder.New),
	}
	f, err := stframework.NewFramework(ctx, registeredPlugins, "",
		frameworkruntime.WithClientSet(fakeClient),
		frameworkruntime.WithEventRecorder(&events.FakeRecorder{}),
		frameworkruntime.WithInformerFactory(informerFactory),
		frameworkruntime.WithSnapshotSharedLister(snapshot),
		frameworkruntime.WithWaitingPods(frameworkruntime.NewWaitingPodsMap()),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		pods             []*corev1.Pod
		cleantrInformer  bool
		expectedEmptyMsg bool
	}{
		{
			name:             "pod does not belong to any task resource",
			pods:             []*corev1.Pod{st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj()},
			cleantrInformer:  true,
			expectedEmptyMsg: true,
		},
		{
			name: "pod belong to task resource tr, but tr does node exist",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			cleantrInformer:  true,
			expectedEmptyMsg: false,
		},
		{
			name: "pod belong to task resource tr, but the member less than MinReservedPods",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedEmptyMsg: false,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleState := framework.NewCycleState()
			if tt.cleantrInformer {
				trInformer.Informer().GetStore().Delete(tr)
				defer trInformer.Informer().GetStore().Add(tr)
			}

			trMgr := core.NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			centralizedScheduling := &KusciaScheduling{trMgr: trMgr, frameworkHandler: f}

			_, code := centralizedScheduling.PreFilter(context.Background(), cycleState, tt.pods[0])
			if code.Message() == "" != tt.expectedEmptyMsg {
				t.Errorf("expectedEmptyMsg %v, got %v", tt.expectedEmptyMsg, code.Message() == "")
			}
		})
	}
}

func TestUnreserve(t *testing.T) {
	tr := util.MakeTaskResource("ns1", "tr", 2, nil)
	tr.UID = "111"
	ctx := context.Background()
	cs := kusciaclientsetfake.NewSimpleClientset(tr)
	trInformerFactory := kusciainformers.NewSharedInformerFactory(cs, 0)
	trInformer := trInformerFactory.Kuscia().V1alpha1().TaskResources()
	trInformerFactory.Start(ctx.Done())

	trInformer.Informer().GetStore().Add(tr)

	fakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	podInformer := informerFactory.Core().V1().Pods()
	nsInformer := informerFactory.Core().V1().Namespaces()
	informerFactory.Start(ctx.Done())

	existingPods, allNodes := util.MakeNodesAndPods(map[string]string{kusciaapisv1alpha1.TaskResourceUID: "111"}, nil, 10, 30)
	snapshot := util.NewFakeSharedLister(existingPods, allNodes)
	// Compose a framework handle.
	// Compose a framework handle.
	registeredPlugins := []stframework.RegisterPluginFunc{
		stframework.RegisterQueueSortPlugin(queuesort.Name, queuesort.New),
		stframework.RegisterBindPlugin(defaultbinder.Name, defaultbinder.New),
	}
	f, err := stframework.NewFramework(ctx, registeredPlugins, "",
		frameworkruntime.WithClientSet(fakeClient),
		frameworkruntime.WithEventRecorder(&events.FakeRecorder{}),
		frameworkruntime.WithInformerFactory(informerFactory),
		frameworkruntime.WithSnapshotSharedLister(snapshot),
		frameworkruntime.WithWaitingPods(frameworkruntime.NewWaitingPodsMap()),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		pods            []*corev1.Pod
		expectedtrExist bool
	}{
		{
			name: "pod1 does not belong to any task resource",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").UID("pod1").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedtrExist: true,
		},
		{
			name: "pod1 belong to task resource tr",
			pods: []*corev1.Pod{
				st.MakePod().Name("pod1").Namespace("ns1").Node("node1").UID("pod1").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj(),
				st.MakePod().Name("pod2").Namespace("ns1").Node("node2").UID("pod2").
					Annotation(kusciaapisv1alpha1.TaskResourceKey, "tr").Label(kusciaapisv1alpha1.TaskResourceUID, "111").Obj()},
			expectedtrExist: false,
		},
	}

	timeout := 10 * time.Second
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycleState := framework.NewCycleState()
			trMgr := core.NewTaskResourceManager(cs, snapshot, trInformer, podInformer, nsInformer, &timeout)
			centralizedScheduling := &KusciaScheduling{trMgr: trMgr, frameworkHandler: f}
			for _, pod := range tt.pods {
				centralizedScheduling.Reserve(context.Background(), cycleState, pod, pod.Spec.NodeName)
			}
			centralizedScheduling.Unreserve(context.Background(), cycleState, tt.pods[0], tt.pods[0].Spec.NodeName)
			trInfo := core.GetTaskResourceInfos(trMgr)
			_, exist := trInfo.Load(tr.Namespace + "/" + tr.Name)
			if exist != tt.expectedtrExist {
				t.Errorf("expectedtrExist %v, got %v", tt.expectedtrExist, exist)
			}
		})
	}
}
