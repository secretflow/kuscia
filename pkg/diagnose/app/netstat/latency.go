// Copyright 2024 Ant Group Co., Ltd.
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

package netstat

import (
	"fmt"
	"time"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"golang.org/x/net/context"
)

const (
	LatencyDuration     = 0
	Iteration           = 100
	LatencyUnit         = "ms"
	DefaultRTTThreshold = 50
)

// LatencyTask records latency between two nodes.
// Client sends request for 100 times and records the average duration.
type LatencyTask struct {
	Client    *client.Client
	threshold int
	output    *TaskOutput
}

func NewLatencyTask(client *client.Client, threshold int) Task {
	if threshold == 0 {
		threshold = DefaultRTTThreshold
	}
	task := &LatencyTask{
		Client:    client,
		threshold: threshold,
		output:    new(TaskOutput),
	}
	task.output.Name = task.Name()
	task.output.Threshold = fmt.Sprintf("%v%v", threshold, LatencyUnit)
	return task
}

func (t *LatencyTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task, threshold: %v", t.Name(), t.threshold)
	defer nlog.Infof("Task %v done, output: %+v", t.Name(), t.output)
	req := &diagnose.MockRequest{
		Duration: LatencyDuration,
	}

	var duration time.Duration
	var err error
	var success int
	for i := 0; i < Iteration; i++ {
		start := time.Now()
		if _, err = t.Client.Mock(ctx, req); err != nil {
			nlog.Errorf("Mock error: %v", err)
		} else {
			duration += time.Since(start)
			success++
		}
	}
	if success == 0 {
		t.output.Result = common.Fail
		t.output.Information = fmt.Sprintf("mock failed, %v", err)
		return
	}
	latency := Decimal(float64(duration.Milliseconds()) / float64(success)) // convert to unit ms
	t.output.DetectedValue = fmt.Sprintf("%v%v", latency, LatencyUnit)
	if latency <= float64(t.threshold) {
		t.output.Result = common.Pass
	} else {
		t.output.Result = common.Warning
		t.output.Information = fmt.Sprintf("not satisfy threshold %v%v", t.threshold, LatencyUnit)
	}
}

func (t *LatencyTask) Output() *TaskOutput {
	return t.output
}

func (t *LatencyTask) Name() string {
	return "RTT"
}
