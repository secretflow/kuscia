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

	"golang.org/x/net/context"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/math"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

const (
	MinRequestBodySize              = 100 << 10 // 100KB
	DefaultRequestBodySizeThreshold = 1         // 1MB
	BodySizeUnit                    = "MB"
)

// ReqSizeTask detects the request body size limitation between two nodes.
// Server returns data of different size in [100, 200, 400, 800, 1024]KB, client checks the status of response to detect the size limitation.
type ReqSizeTask struct {
	client    *client.Client
	threshold int
	output    *TaskOutput
}

func NewReqSizeTask(client *client.Client, threshold int) Task {
	if threshold == 0 {
		threshold = DefaultRequestBodySizeThreshold
	}
	task := &ReqSizeTask{
		client:    client,
		threshold: MBToByte(float64(threshold)),
		output:    new(TaskOutput),
	}
	task.output.Name = task.Name()
	task.output.Threshold = fmt.Sprintf("%v%v", threshold, BodySizeUnit)
	return task
}

func (t *ReqSizeTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task, threshold: %v", t.Name(), t.output.Threshold)
	testcases := make([]int, 0)
	for i := MinRequestBodySize; i < t.threshold; i = i * 2 {
		testcases = append(testcases, i)
	}
	testcases = append(testcases, t.threshold)
	reqBodySize, errMsg := ParallelSearchArray(ctx, testcases, t.Detect)
	if reqBodySize == 0 {
		t.output.Result = common.Fail
		t.output.Information = fmt.Sprintf("mock request failed, %v", errMsg)
	} else if reqBodySize == t.threshold {
		t.output.Result = common.Pass
		t.output.DetectedValue = fmt.Sprintf(">%v", math.ByteCountBinary(int64(reqBodySize)))
	} else if reqBodySize < t.threshold && reqBodySize >= min(t.threshold, MinRequestBodySize) {
		t.output.Result = common.Warning
		t.output.DetectedValue = fmt.Sprintf("%v~%v", math.ByteCountBinary(int64(reqBodySize)), math.ByteCountBinary(int64(min(t.threshold, 2*reqBodySize))))
		t.output.Information = fmt.Sprintf("not satisfy threshold %v", math.ByteCountBinary(int64(t.threshold)))
	} else {
		t.output.Result = common.Warning
		t.output.DetectedValue = fmt.Sprintf("<%v", math.ByteCountBinary(int64(min(t.threshold, 2*reqBodySize))))
		t.output.Information = fmt.Sprintf("not satisfy threshold %v", math.ByteCountBinary(int64(t.threshold)))
	}
}

func (t *ReqSizeTask) Detect(ctx context.Context, reqSize int) error {
	// mock request, set request body to <reqSize>
	req := &diagnose.MockRequest{
		Data: make([]byte, reqSize),
	}
	if _, err := t.client.Mock(ctx, req); err != nil {
		return err
	}
	return nil
}

func (t *ReqSizeTask) Output() *TaskOutput {
	return t.output
}

func (t *ReqSizeTask) Name() string {
	return "REQUEST_BODY_SIZE"
}
