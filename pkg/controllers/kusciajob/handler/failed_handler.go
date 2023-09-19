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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/controllers/kusciajob/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// FailedHandler will handle kuscia job in Failed phase.
type FailedHandler struct {
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
	namespaceLister  corelisters.NamespaceLister
	recorder         record.EventRecorder
}

// NewFailedHandler return FailedHandler to handle Failed kuscia job.
func NewFailedHandler(deps *Dependencies) *FailedHandler {
	return &FailedHandler{
		recorder:         deps.Recorder,
		kusciaTaskLister: deps.KusciaTaskLister,
		namespaceLister:  deps.NamespaceLister,
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
// It will do some tail-in work when the job phase is failed.
func (h *FailedHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	asInitiator := false
	if utilsres.SelfClusterAsInitiator(h.namespaceLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
		asInitiator = true
	}

	allTaskFinished := true
	for taskID, phase := range kusciaJob.Status.TaskStatus {
		if phase != kusciaapisv1alpha1.TaskFailed && phase != kusciaapisv1alpha1.TaskSucceeded {
			if !asInitiator {
				kusciaJob.Status.TaskStatus[taskID] = kusciaapisv1alpha1.TaskFailed
				continue
			}

			task, err := h.kusciaTaskLister.Get(taskID)
			if err != nil {
				nlog.Warnf("Get kuscia task %v failed, %v", taskID, err)
				kusciaJob.Status.TaskStatus[taskID] = kusciaapisv1alpha1.TaskFailed
				needUpdate = true
				continue
			}

			if task.Status.Phase != kusciaapisv1alpha1.TaskFailed && task.Status.Phase != kusciaapisv1alpha1.TaskSucceeded {
				allTaskFinished = false
			}

			if phase != task.Status.Phase {
				kusciaJob.Status.TaskStatus[taskID] = task.Status.Phase
				needUpdate = true
			}
		}
	}

	if allTaskFinished {
		if kusciaJob.Status.CompletionTime == nil || !kusciaJob.Status.CompletionTime.Equal(&now) {
			kusciaJob.Status.CompletionTime = &now
			needUpdate = true
		}
	}

	metrics.JobResultStats.WithLabelValues(metrics.Failed).Inc()
	return needUpdate, nil
}
