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

// BufferTask checks if there's any proxy buffer between two nodes.
// Server returns 512B chunked data every 50ms for 10s, which returns 100KB in total. Client records the size of the first data it receives
// if the size is larger than 512B, we assume there's proxy buffer.
type BufferTask struct {
	client *client.Client
	output *TaskOutput
}

const (
	DefaultBufferSize = 1 << 9 // 512B
	ChunkedInterval   = 50     // 50ms
	BufferUnit        = "bytes"
)

func NewBufferTask(client *client.Client) Task {
	task := &BufferTask{
		client: client,
		output: new(TaskOutput),
	}
	task.output.Name = task.Name()
	return task
}

func (t *BufferTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task", t.Name())
	// mock request, server returns 512B data every 50ms for 10s, which returns 100KB in total
	req := &diagnose.MockRequest{
		EnableChunked:   true,
		ChunkedSize:     DefaultBufferSize,
		ChunkedInterval: ChunkedInterval,
		Duration:        Duration,
	}
	url := fmt.Sprintf("http://%v/%v/%v", t.client.HostName, common.DiagnoseNetworkGroup, common.DiagnoseMockPath)
	resp, err := t.client.MockChunk(req, url)
	if err != nil {
		t.output.Result = common.Fail
		t.output.Information = err.Error()
		return
	}
	defer resp.Body.Close()

	buffering, bufferSize := t.RecordBuffer(resp)
	if buffering {
		t.output.Result = common.Fail
		t.output.DetectedValue = fmt.Sprintf("%v%v", bufferSize, BufferUnit)
		t.output.Information = "proxy buffer exists, please check the proxy config"
	} else {
		t.output.Result = common.Pass
		t.output.DetectedValue = "N/A"
	}
}

func (t *BufferTask) RecordBuffer(resp *http.Response) (bool, int) {
	buffering := false
	bufferSize := 0
	buffer := make([]byte, DefaultBufferSize*Duration/ChunkedInterval) // 100kb data buffer
	for {
		cnt, err := resp.Body.Read(buffer)
		if err != nil {
			break
		}
		if cnt != 0 {
			if cnt > DefaultBufferSize {
				buffering = true
				bufferSize = cnt
				break
			}
		}
	}
	return buffering, bufferSize
}

func (t *BufferTask) Output() *TaskOutput {
	return t.output
}

func (t *BufferTask) Name() string {
	return "PROXY_BUFFER"
}
