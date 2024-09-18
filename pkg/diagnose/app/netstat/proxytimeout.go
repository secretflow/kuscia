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

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"golang.org/x/net/context"
)

const (
	MinProxyTimeout              = 10000 // 10s
	DefaultProxyTimeoutThreshold = 600   // 10min
	ProxyTimeoutUnit             = "s"
)

// ProxyTimeoutTask detects the proxy read timeout between two nodes.
// Server returns data after [10, 20, 40, 80, 160, 240, 480, 600]s, client checks the status of response to detect the proxytimeout.
type ProxyTimeoutTask struct {
	client    *client.Client
	threshold int
	output    *TaskOutput
}

func NewProxyTimeoutTask(client *client.Client, threshold int) Task {
	if threshold == 0 {
		threshold = DefaultProxyTimeoutThreshold
	}
	task := &ProxyTimeoutTask{
		client:    client,
		threshold: ToMillisecond(float64(threshold)),
		output:    new(TaskOutput),
	}
	task.output.Name = task.Name()
	task.output.Threshold = fmt.Sprintf("%v%v", threshold, ProxyTimeoutUnit)
	return task
}

func (t *ProxyTimeoutTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task, threshold: %v", t.Name(), t.threshold)
	defer nlog.Infof("Task %v done, output: %+v", t.Name(), t.output)
	testcases := make([]int, 0)
	for i := MinProxyTimeout; i < t.threshold; i = i * 2 {
		testcases = append(testcases, i)
	}
	testcases = append(testcases, t.threshold)
	proxyTimeout, errMsg := ParallelSearchArray(ctx, testcases, t.Detect)
	if proxyTimeout == 0 {
		t.output.Result = common.Warning
		t.output.Information = fmt.Sprintf("all mock request failed, proxy timeout may smaller than 10s, %v", errMsg)
	} else if proxyTimeout == t.threshold {
		t.output.Result = common.Pass
		t.output.DetectedValue = fmt.Sprintf(">%v%v", ToSecond(proxyTimeout), ProxyTimeoutUnit)
	} else if proxyTimeout < t.threshold && proxyTimeout >= min(t.threshold, MinProxyTimeout) {
		t.output.Result = common.Warning
		t.output.DetectedValue = fmt.Sprintf("%v~%v%v", ToSecond(proxyTimeout), ToSecond(min(t.threshold, 2*proxyTimeout)), ProxyTimeoutUnit)
		t.output.Information = fmt.Sprintf("not satisfy threshold %v%v", ToSecond(t.threshold), ProxyTimeoutUnit)
	} else {
		t.output.Result = common.Warning
		t.output.DetectedValue = fmt.Sprintf("<%v%v", ToSecond(min(t.threshold, MinProxyTimeout)), ProxyTimeoutUnit)
		t.output.Information = fmt.Sprintf("not satisfy threshold %v%v", ToSecond(t.threshold), ProxyTimeoutUnit)
	}
}

func (t *ProxyTimeoutTask) Detect(ctx context.Context, timeout int) error {
	// mock request, server return after <timeout>
	req := &diagnose.MockRequest{
		Duration: int64(timeout),
	}
	if _, err := t.client.Mock(ctx, req); err != nil {
		return err
	}
	return nil
}

func (t *ProxyTimeoutTask) Output() *TaskOutput {
	return t.output
}

func (t *ProxyTimeoutTask) Name() string {
	return "PROXY_TIMEOUT"
}
