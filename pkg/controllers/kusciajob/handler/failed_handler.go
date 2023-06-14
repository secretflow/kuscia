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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/controllers/kusciajob/metrics"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// FailedHandler will handle kuscia job in Failed phase.
type FailedHandler struct {
	recorder record.EventRecorder
}

// NewFailedHandler return FailedHandler to handle Failed kuscia job.
func NewFailedHandler(recorder record.EventRecorder) *FailedHandler {
	return &FailedHandler{
		recorder: recorder,
	}
}

// HandlePhase implements the KusciaJobPhaseHandler interface.
// It will do some tail-in work when the job phase is failed.
func (s *FailedHandler) HandlePhase(kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	s.recorder.Event(kusciaJob, v1.EventTypeWarning, "KusciaJobFailed", "KusciaJob failed to run")
	now := metav1.Now()
	kusciaJob.Status.CompletionTime = &now
	kusciaJob.Status.LastReconcileTime = &now
	metrics.JobResultStats.WithLabelValues(metrics.Failed).Inc()
	return true, nil
}
