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
	"k8s.io/client-go/tools/record"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

// Dependencies defines KusciaJobPhaseHandlerFactory's dependencies.
type Dependencies struct {
	Recorder         record.EventRecorder
	KusciaClient     versioned.Interface
	KusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
}

// KusciaJobPhaseHandler defines that how to handle the kuscia job in each phase.
type KusciaJobPhaseHandler interface {
	HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error)
}

// NewKusciaJobPhaseHandlerFactory return a state machine to handle the kuscia job in each phase.
func NewKusciaJobPhaseHandlerFactory(deps *Dependencies) *KusciaJobPhaseHandlerFactory {
	createdHandler := NewPendingHandler(deps)
	runningHandler := NewRunningHandler(deps)
	succeededHandler := NewSucceededHandler(deps.Recorder)
	failedHandler := NewFailedHandler(deps.Recorder)
	KusciaJobStateHandlerMap := map[kusciaapisv1alpha1.KusciaJobPhase]KusciaJobPhaseHandler{
		kusciaapisv1alpha1.KusciaJobPending:   createdHandler,
		kusciaapisv1alpha1.KusciaJobRunning:   runningHandler,
		kusciaapisv1alpha1.KusciaJobSucceeded: succeededHandler,
		kusciaapisv1alpha1.KusciaJobFailed:    failedHandler,
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
