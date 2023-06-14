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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Succeeded = "succeeded"
	Failed    = "failed"
)

var (
	TaskRequeueCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kuscia_task_requeue_count",
		Help: "Counts number of kuscia tasks requeue",
	}, []string{"task_name"})

	WorkerQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kuscia_worker_queue_size",
		Help: "Size of kusciatask worker queue",
	})

	SyncDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "kuscia_task_sync_durations_seconds",
			Help:       "Sync latency distributions of kuscia tasks.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"condition", "status"},
	)

	TaskResultStats = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kuscia_task_result_stats",
			Help: "Counts number of successful or failed kuscia tasks",
		},
		[]string{"result"},
	)
)

func ClearDeadMetrics(key string) {
	TaskRequeueCount.DeleteLabelValues(key)
}
