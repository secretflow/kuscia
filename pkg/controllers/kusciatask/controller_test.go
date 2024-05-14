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

package kusciatask

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func makeTestAppImage() *kusciaapisv1alpha1.AppImage {
	return &kusciaapisv1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-image-1",
		},
		Spec: kusciaapisv1alpha1.AppImageSpec{
			Image: kusciaapisv1alpha1.AppImageInfo{
				Name: "test-image",
				Tag:  "1.0.0",
			},
			DeployTemplates: []kusciaapisv1alpha1.DeployTemplate{
				{
					Name: "abc",
					Role: "server,client",
					Spec: kusciaapisv1alpha1.PodSpec{
						Containers: []kusciaapisv1alpha1.Container{
							{
								Name:    "container-0",
								Command: []string{"ls"},
							},
						},
					},
				},
			},
		},
	}
}

func makeTestKusciaTask(phase kusciaapisv1alpha1.KusciaTaskPhase) *kusciaapisv1alpha1.KusciaTask {
	return &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kusciatask-001",
			Namespace: common.KusciaCrossDomain,
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			TaskInputConfig: "task input config",
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "domain-a",
					AppImageRef: "test-image-1",
					Role:        "server",
				},
				{
					DomainID:    "domain-b",
					AppImageRef: "test-image-1",
					Role:        "client",
				},
			},
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{Phase: phase},
	}
}

func TestNewController(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	assert.NotNil(t, c)
}

func TestControllerRun(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	go func() {
		time.Sleep(1 * time.Second)
		c.Stop()
	}()
	err := c.Run(4)
	assert.Nil(t, err)
}

func TestEnqueueKusciaTask(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)
	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskPending)
	cc.enqueueKusciaTask(kt)
	assert.Equal(t, 1, cc.taskQueue.Len())
}

func TestHandleTaskResourceGroupObject(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)

	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	ktInformer.Informer().GetStore().Add(kt)
	cc.kusciaTaskLister = ktInformer.Lister()

	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trg1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 2,
			Initiator:          "domain-a",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "domain-a",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "test-image-1-server-0",
						},
					},
				},
			},
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kusciatask-001",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 2,
			Initiator:          "domain-a",
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "domain-a",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "test-image-1-server-0",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "invalid obj type",
			obj:  "test",
			want: 0,
		},
		{
			name: "kuscia task is not found",
			obj:  trg1,
			want: 0,
		},
		{
			name: "enqueue kuscia task",
			obj:  trg2,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleTaskResourceGroupObject(tt.obj)
			assert.Equal(t, tt.want, cc.taskQueue.Len())
		})
	}
}

func TestHandlePodObject(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)

	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	ktInformer.Informer().GetStore().Add(kt)
	cc.kusciaTaskLister = ktInformer.Lister()

	pod1 := st.MakePod().Name("pod1").Obj()
	pod2 := st.MakePod().Name("pod2").Annotation(common.TaskIDAnnotationKey, kt.Name).Obj()

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "invalid obj type",
			obj:  "test",
			want: 0,
		},
		{
			name: "invalid owner reference",
			obj:  pod1,
			want: 0,
		},
		{
			name: "enqueue kuscia task",
			obj:  pod2,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handlePodObject(tt.obj)
			assert.Equal(t, tt.want, cc.taskQueue.Len())
		})
	}
}

func TestProcessNextWorkItem(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)

	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	ktInformer.Informer().GetStore().Add(kt)

	tests := []struct {
		name     string
		shutdown bool
		obj      interface{}
		want     bool
	}{
		{
			name: "object is invalid",
			obj:  kt,
			want: true,
		},
		{
			name: "object is valid",
			obj:  kt.Name,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj != "" {
				cc.taskQueue.Add(tt.obj)
			}

			got := cc.processNextWorkItem()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFailKusciaTask(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)

	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)

	cc.failKusciaTask(kt, fmt.Errorf("task failed"))
	assert.Equal(t, kusciaapisv1alpha1.TaskFailed, kt.Status.Phase)
}

func TestUpdateTaskStatus(t *testing.T) {
	pendingKt := makeTestKusciaTask(kusciaapisv1alpha1.TaskPending)
	runningKt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset(runningKt)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	cc := c.(*Controller)

	got := cc.updateTaskStatus(pendingKt, runningKt)
	assert.Nil(t, got)
}

func TestName(t *testing.T) {
	kt := makeTestKusciaTask(kusciaapisv1alpha1.TaskRunning)
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset(kt)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeFakeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciataskcontroller"})
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: eventRecorder,
	})
	got := c.Name()
	assert.Equal(t, controllerName, got)
}
