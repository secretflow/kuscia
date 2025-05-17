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
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"github.com/xhd2015/xgo/runtime/mock"
	"golang.org/x/net/context"
	"gotest.tools/v3/assert"
)

func TestLatency(t *testing.T) {
	tests := []struct {
		name    string
		latency int
		result  string
	}{
		{"latency_30ms_Warning", 30, common.Warning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := client.NewDiagnoseClient("")
			task := NewLatencyTask(cli, 20)
			latencyTask := task.(*LatencyTask)

			mock.Patch(cli.Mock, func(ctx context.Context, request *diagnose.MockRequest) (response *diagnose.MockResponse, err error) {
				time.Sleep(time.Duration(tt.latency) * time.Millisecond)
				return nil, nil
			})
			latencyTask.Run(context.Background())
			assert.Equal(t, latencyTask.output.Result, tt.result)
		})
	}
}
