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
	"net/http"

	"golang.org/x/net/context"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

const (
	ChunkedSize               = 100 << 10 // 100KB
	Duration                  = 10000     // 10s
	DefaultBandWidthThreshold = 10        // 10Mbits/sec
	BandWidthUnit             = "Mbits/sec"
)

// BandWidthTask records bandwidth between two nodes.
// Server returns 100kb chunked data continuously for 10s, client records the total size of data that it actually receives.
type BandWidthTask struct {
	client    *client.Client
	threshold int
	output    *TaskOutput
}

func NewBandWidthTask(client *client.Client, threshold int) Task {
	if threshold == 0 {
		threshold = DefaultBandWidthThreshold
	}
	task := &BandWidthTask{
		client:    client,
		threshold: threshold,
		output:    new(TaskOutput),
	}
	task.output.Name = task.Name()
	task.output.Threshold = fmt.Sprintf("%v%v", threshold, BandWidthUnit)
	return task
}

func (t *BandWidthTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task, threshold: %v", t.Name(), t.output.Threshold)
	// mock request, server return 100kb chunked data continuously for 10s
	req := &diagnose.MockRequest{
		ChunkedSize:   ChunkedSize,
		Duration:      Duration,
		EnableChunked: true,
	}

	url := fmt.Sprintf("http://%v/%v/%v", t.client.HostName, common.DiagnoseNetworkGroup, common.DiagnoseMockPath)
	resp, err := t.client.MockChunk(req, url)
	if err != nil {
		t.output.Result = common.Fail
		t.output.Information = err.Error()
		return
	}
	defer resp.Body.Close()

	size := t.RecordData(resp)
	result := ToMbps(size) // convert to Mbps
	t.output.DetectedValue = fmt.Sprintf("%v%v", result, BandWidthUnit)
	if result > float64(t.threshold) {
		t.output.Result = common.Pass
	} else {
		t.output.Result = common.Warning
		t.output.Information = fmt.Sprintf("not satisfy threshold %v%v", t.threshold, BandWidthUnit)
	}
}

func (t *BandWidthTask) RecordData(resp *http.Response) int {
	size := 0
	buffer := make([]byte, ChunkedSize)
	for {
		recvSize, err := resp.Body.Read(buffer)
		if err != nil {
			break
		}
		size += recvSize
	}
	return size / int(ToSecond(Duration))
}

func (t *BandWidthTask) Output() *TaskOutput {
	return t.output
}

func (t *BandWidthTask) Name() string {
	return "BANDWIDTH"
}
