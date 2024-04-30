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
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

// Dependencies defines KusciaJobPhaseHandlerFactory's dependencies.
type Dependencies struct {
	Recorder              record.EventRecorder
	KusciaClient          versioned.Interface
	KusciaTaskLister      kuscialistersv1alpha1.KusciaTaskLister
	NamespaceLister       corelisters.NamespaceLister
	DomainLister          kuscialistersv1alpha1.DomainLister
	EnableWorkloadApprove bool
}

// KusciaJobPhaseHandler defines that how to handle the kuscia job in each phase.
type KusciaJobPhaseHandler interface {
	HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error)
}

// NewKusciaJobPhaseHandlerFactory return a state machine to handle the kuscia job in each phase.
func NewKusciaJobPhaseHandlerFactory(deps *Dependencies) *KusciaJobPhaseHandlerFactory {

	KusciaJobStateHandlerMap := map[kusciaapisv1alpha1.KusciaJobPhase]KusciaJobPhaseHandler{
		kusciaapisv1alpha1.KusciaJobInitialized:      NewInitializedHandler(deps),
		kusciaapisv1alpha1.KusciaJobAwaitingApproval: NewAwaitingApprovalHandler(deps),
		kusciaapisv1alpha1.KusciaJobPending:          NewPendingHandler(deps),
		kusciaapisv1alpha1.KusciaJobRunning:          NewRunningHandler(deps),
		kusciaapisv1alpha1.KusciaJobSuspended:        NewSuspendedHandler(deps),
		kusciaapisv1alpha1.KusciaJobSucceeded:        NewSucceededHandler(deps),
		kusciaapisv1alpha1.KusciaJobFailed:           NewFailedHandler(deps),
		kusciaapisv1alpha1.KusciaJobApprovalReject:   NewApprovalRejectHandler(deps),
		kusciaapisv1alpha1.KusciaJobCancelled:        NewCancelledHandler(deps),
	}
	return &KusciaJobPhaseHandlerFactory{KusciaTaskStateHandlerMap: KusciaJobStateHandlerMap}
}

// KusciaJobPhaseHandlerFactory is a state machine to handle the kuscia job in each phase.
type KusciaJobPhaseHandlerFactory struct {
	KusciaTaskStateHandlerMap map[kusciaapisv1alpha1.KusciaJobPhase]KusciaJobPhaseHandler
}

// KusciaJobPhaseHandlerFor get handler for phase.
func (m *KusciaJobPhaseHandlerFactory) KusciaJobPhaseHandlerFor(phase kusciaapisv1alpha1.KusciaJobPhase) KusciaJobPhaseHandler {
	return m.KusciaTaskStateHandlerMap[phase]
}
