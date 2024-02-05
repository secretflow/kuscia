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

package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestRunningHandler_Handle(t *testing.T) {
	trg1 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task-1",
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers: 2,
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID: "ns-a",
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod-01",
						},
					},
					MinReservedPods: 1,
				},
			},
			OutOfControlledParties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					DomainID:        "ns-b",
					MinReservedPods: 1,
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: "pod-01",
						},
					},
				},
			},
		},
	}

	trg2 := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task-2",
		},
		Status: kusciaapisv1alpha1.TaskResourceGroupStatus{
			Phase: kusciaapisv1alpha1.TaskResourceGroupPhaseFailed,
		},
	}

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{common.TaskResourceGroupAnnotationKey: "task"},
			Labels:      map[string]string{common.LabelTaskResourceGroupUID: "1111"},
			Name:        "pod-01",
			Namespace:   "ns-a",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{common.TaskResourceGroupAnnotationKey: "task"},
			Labels:      map[string]string{common.LabelTaskResourceGroupUID: "1111"},
			Name:        "pod-01",
			Namespace:   "ns-b",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	kt1 := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task-1",
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{
			PodStatuses: map[string]*kusciaapisv1alpha1.PodStatus{
				"ns-a/pod-01": {
					Namespace: "ns-a",
					PodName:   "pod-01",
				},
				"ns-b/pod-01": {
					Namespace: "ns-b",
					PodName:   "pod-01",
				},
			},
			Phase: kusciaapisv1alpha1.TaskRunning,
		},
	}

	kt2 := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task-2",
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{
			PodStatuses: map[string]*kusciaapisv1alpha1.PodStatus{
				"ns-a/pod-01": {
					Namespace: "ns-a",
					PodName:   "pod-01",
				},
				"ns-b/pod-01": {
					Namespace: "ns-b",
					PodName:   "pod-01",
				},
			},
			Phase: kusciaapisv1alpha1.TaskRunning,
		},
	}

	tests := []struct {
		name       string
		kusciatask *kusciaapisv1alpha1.KusciaTask
		trg        *kusciaapisv1alpha1.TaskResourceGroup
		pods       []runtime.Object
		wantPhase  kusciaapisv1alpha1.KusciaTaskPhase
	}{
		{
			name:       "update kuscia task ResourceReady condition",
			kusciatask: kt1,
			trg:        trg1,
			pods: []runtime.Object{
				pod1,
				pod2,
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name:       "task resource group phase failed",
			kusciatask: kt2,
			trg:        trg2,
			pods: []runtime.Object{
				pod1,
				pod2,
			},
			wantPhase: kusciaapisv1alpha1.TaskFailed,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("TestCase %d", i), func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset(tt.pods...)
			kusciaClient := kusciafake.NewSimpleClientset()
			kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			nsInformer := kubeInformersFactory.Core().V1().Namespaces()
			podInformer := kubeInformersFactory.Core().V1().Pods()
			for _, ttp := range tt.pods {
				podInformer.Informer().GetStore().Add(ttp)
			}

			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
			trgInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups()
			trgInformer.Informer().GetStore().Add(tt.trg)

			go kubeInformersFactory.Start(wait.NeverStop)
			deps := Dependencies{
				KubeClient:       kubeClient,
				KusciaClient:     kusciaClient,
				TrgLister:        trgInformer.Lister(),
				NamespacesLister: nsInformer.Lister(),
				PodsLister:       podInformer.Lister(),
			}

			h := NewRunningHandler(&deps)
			_, err := h.Handle(tt.kusciatask)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPhase, tt.kusciatask.Status.Phase)
		})
	}
}

func makeTaskResourceGroup(name string, partiesDomainID []string,
	outOfControlledParties []string) *kusciaapisv1alpha1.TaskResourceGroup {
	trg := kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			Initiator:          partiesDomainID[0],
			MinReservedMembers: 1,
			Parties: []kusciaapisv1alpha1.TaskResourceGroupParty{
				{
					Role:     "host",
					DomainID: partiesDomainID[0],
					Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
						{
							Name: partiesDomainID[0],
						},
					},
					TaskResourceName: partiesDomainID[0],
				},
			},
		},
	}

	if len(partiesDomainID) > 1 {
		trg.Spec.MinReservedMembers++
		party := kusciaapisv1alpha1.TaskResourceGroupParty{
			Role:     "guest",
			DomainID: partiesDomainID[1],
			Pods: []kusciaapisv1alpha1.TaskResourceGroupPartyPod{
				{
					Name: partiesDomainID[1],
				},
			},
			TaskResourceName: partiesDomainID[1],
		}
		trg.Spec.Parties = append(trg.Spec.Parties, party)
	}

	if len(outOfControlledParties) > 0 {
		trg.Spec.OutOfControlledParties = make([]kusciaapisv1alpha1.TaskResourceGroupParty, 0)
		for _, domainID := range outOfControlledParties {
			party := kusciaapisv1alpha1.TaskResourceGroupParty{
				Role:     "guest",
				DomainID: domainID,
			}
			trg.Spec.OutOfControlledParties = append(trg.Spec.OutOfControlledParties, party)
		}
	}
	return &trg
}

func makePod(name string, phase v1.PodPhase) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
		Status: v1.PodStatus{
			Phase: phase,
		},
	}
}

func makeNamespace(name string, labels map[string]string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func TestReconcileTaskStatus(t *testing.T) {
	tests := []struct {
		name       string
		taskStatus *kusciaapisv1alpha1.KusciaTaskStatus
		namespaces []*v1.Namespace
		trg        *kusciaapisv1alpha1.TaskResourceGroup
		pods       []*v1.Pod
		wantPhase  kusciaapisv1alpha1.KusciaTaskPhase
	}{
		{
			name: "all party are pending and task does not expired",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				Phase: kusciaapisv1alpha1.TaskRunning,
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(-10 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodPending),
				makePod("bob", v1.PodPending),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name:       "one party is running and another is pending",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{},
			trg:        makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodRunning),
				makePod("bob", v1.PodPending),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name:       "all party are running",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{},
			trg:        makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodRunning),
				makePod("bob", v1.PodRunning),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name:       "one party is succeeded and another is running",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{},
			trg:        makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodRunning),
				makePod("bob", v1.PodSucceeded),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name:       "all party are succeeded",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{},
			trg:        makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodSucceeded),
				makePod("bob", v1.PodSucceeded),
			},
			wantPhase: kusciaapisv1alpha1.TaskSucceeded,
		},
		//{
		//	name: "interconn task, self cluster as initiator, interconn party is pending, another is pending, task does not expired",
		//	taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
		//		Phase: kusciaapisv1alpha1.TaskRunning,
		//		PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
		//			{
		//				DomainID: "bob",
		//				Role:     "guest",
		//				Phase:    kusciaapisv1alpha1.TaskPending,
		//			},
		//		},
		//		StartTime: func() *metav1.Time {
		//			now := metav1.Now().Add(100 * time.Second)
		//			return &metav1.Time{Time: now}
		//		}(),
		//	},
		//	namespaces: []*v1.Namespace{
		//		makeNamespace("alice", nil),
		//		makeNamespace("bob", map[string]string{
		//			common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
		//			common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
		//	},
		//	trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
		//	pods: []*v1.Pod{
		//		makePod("alice", v1.PodPending),
		//	},
		//	wantPhase: kusciaapisv1alpha1.TaskRunning,
		//},
		//{
		//	name: "interconn task, self cluster as initiator, interconn party is pending, another is running",
		//	taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
		//		Phase: kusciaapisv1alpha1.TaskRunning,
		//		PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
		//			{
		//				DomainID: "bob",
		//				Role:     "guest",
		//				Phase:    kusciaapisv1alpha1.TaskPending,
		//			},
		//		},
		//		StartTime: func() *metav1.Time {
		//			now := metav1.Now().Add(-800 * time.Second)
		//			return &metav1.Time{Time: now}
		//		}(),
		//	},
		//	namespaces: []*v1.Namespace{
		//		makeNamespace("alice", nil),
		//		makeNamespace("bob", map[string]string{
		//			common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
		//			common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
		//	},
		//	trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
		//	pods: []*v1.Pod{
		//		makePod("alice", v1.PodRunning),
		//	},
		//	wantPhase: kusciaapisv1alpha1.TaskRunning,
		//},
		//{
		//	name: "interconn task, self cluster as initiator, interconn party is running, another is running",
		//	taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
		//		Phase: kusciaapisv1alpha1.TaskRunning,
		//		PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
		//			{
		//				DomainID: "bob",
		//				Role:     "guest",
		//				Phase:    kusciaapisv1alpha1.TaskRunning,
		//			},
		//		},
		//		StartTime: func() *metav1.Time {
		//			now := metav1.Now().Add(-800 * time.Second)
		//			return &metav1.Time{Time: now}
		//		}(),
		//	},
		//	namespaces: []*v1.Namespace{
		//		makeNamespace("alice", nil),
		//		makeNamespace("bob", map[string]string{
		//			common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
		//			common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
		//	},
		//	trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
		//	pods: []*v1.Pod{
		//		makePod("alice", v1.PodRunning),
		//	},
		//	wantPhase: kusciaapisv1alpha1.TaskRunning,
		//},
		{
			name: "interconn task, self cluster as initiator, interconn party is failed, another is running",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				Phase: kusciaapisv1alpha1.TaskRunning,
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskFailed,
					},
				},
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodRunning),
			},
			wantPhase: kusciaapisv1alpha1.TaskFailed,
		},
		{
			name: "interconn task, self cluster as initiator, interconn party is failed, another is failed",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				Phase: kusciaapisv1alpha1.TaskRunning,
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskFailed,
					},
				},
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodFailed),
			},
			wantPhase: kusciaapisv1alpha1.TaskFailed,
		},
		//{
		//	name: "interconn task, self cluster as initiator, interconn party is running, another is succeeded",
		//	taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
		//		Phase: kusciaapisv1alpha1.TaskRunning,
		//		PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
		//			{
		//				DomainID: "bob",
		//				Role:     "guest",
		//				Phase:    kusciaapisv1alpha1.TaskRunning,
		//			},
		//		},
		//		StartTime: func() *metav1.Time {
		//			now := metav1.Now().Add(200 * time.Second)
		//			return &metav1.Time{Time: now}
		//		}(),
		//	},
		//	namespaces: []*v1.Namespace{
		//		makeNamespace("alice", nil),
		//		makeNamespace("bob", map[string]string{
		//			common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
		//			common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
		//	},
		//	trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
		//	pods: []*v1.Pod{
		//		makePod("alice", v1.PodSucceeded),
		//	},
		//	wantPhase: kusciaapisv1alpha1.TaskRunning,
		//},
		{
			name: "interconn task, self cluster as initiator, interconn party is succeeded, another is succeeded",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				Phase: kusciaapisv1alpha1.TaskRunning,
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskSucceeded,
					},
				},
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice", "bob"}, nil),
			pods: []*v1.Pod{
				makePod("alice", v1.PodSucceeded),
			},
			wantPhase: kusciaapisv1alpha1.TaskSucceeded,
		},
		{
			name: "interconn task, self cluster is not initiator, self cluster task is pending, task does not expired",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				Phase: kusciaapisv1alpha1.TaskRunning,
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskRunning,
					},
				},
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice"}, []string{"bob"}),
			pods: []*v1.Pod{
				makePod("alice", v1.PodPending),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name: "interconn task, self cluster is not initiator, self cluster task is running",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskRunning,
					},
				},
				Phase: kusciaapisv1alpha1.TaskPending,
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice"}, []string{"bob"}),
			pods: []*v1.Pod{
				makePod("alice", v1.PodRunning),
			},
			wantPhase: kusciaapisv1alpha1.TaskRunning,
		},
		{
			name: "interconn task, self cluster is not initiator, self cluster task is succeeded",
			taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
				PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
					{
						DomainID: "bob",
						Role:     "guest",
						Phase:    kusciaapisv1alpha1.TaskSucceeded,
					},
				},
				Phase: kusciaapisv1alpha1.TaskRunning,
				StartTime: func() *metav1.Time {
					now := metav1.Now().Add(200 * time.Second)
					return &metav1.Time{Time: now}
				}(),
			},
			namespaces: []*v1.Namespace{
				makeNamespace("alice", nil),
				makeNamespace("bob", map[string]string{
					common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
					common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
			},
			trg: makeTaskResourceGroup("trg-1", []string{"alice"}, []string{"bob"}),
			pods: []*v1.Pod{
				makePod("alice", v1.PodSucceeded),
			},
			wantPhase: kusciaapisv1alpha1.TaskSucceeded,
		},
		//{
		//	name: "interconn task, self cluster is not initiator, self cluster task is failed",
		//	taskStatus: &kusciaapisv1alpha1.KusciaTaskStatus{
		//		PartyTaskStatus: []kusciaapisv1alpha1.PartyTaskStatus{
		//			{
		//				DomainID: "bob",
		//				Role:     "guest",
		//				Phase:    kusciaapisv1alpha1.TaskSucceeded,
		//			},
		//		},
		//		Phase: kusciaapisv1alpha1.TaskRunning,
		//		StartTime: func() *metav1.Time {
		//			now := metav1.Now().Add(200 * time.Second)
		//			return &metav1.Time{Time: now}
		//		}(),
		//	},
		//	namespaces: []*v1.Namespace{
		//		makeNamespace("alice", nil),
		//		makeNamespace("bob", map[string]string{
		//			common.LabelDomainRole:         string(kusciaapisv1alpha1.Partner),
		//			common.LabelInterConnProtocols: string(kusciaapisv1alpha1.InterConnBFIA)}),
		//	},
		//	trg: makeTaskResourceGroup("trg-1", []string{"alice"}, []string{"bob"}),
		//	pods: []*v1.Pod{
		//		makePod("alice", v1.PodFailed),
		//	},
		//	wantPhase: kusciaapisv1alpha1.TaskFailed,
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset()
			kusciaClient := kusciafake.NewSimpleClientset()
			kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			nsInformer := kubeInformersFactory.Core().V1().Namespaces()
			podInformer := kubeInformersFactory.Core().V1().Pods()

			h := &RunningHandler{
				kubeClient:   kubeClient,
				kusciaClient: kusciaClient,
				podsLister:   podInformer.Lister(),
				nsLister:     nsInformer.Lister(),
			}

			if tt.pods != nil {
				for i := range tt.pods {
					podInformer.Informer().GetStore().Add(tt.pods[i])
				}
			}

			if tt.namespaces != nil {
				for i := range tt.namespaces {
					nsInformer.Informer().GetStore().Add(tt.namespaces[i])
				}
			}

			h.reconcileTaskStatus(tt.taskStatus, tt.trg)
			assert.Equal(t, tt.wantPhase, tt.taskStatus.Phase)
		})
	}
}
