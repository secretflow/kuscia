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
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey"
	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"golang.org/x/net/context"
	"gotest.tools/v3/assert"
)

func TestBandWidth(t *testing.T) {
	tests := []struct {
		name    string
		latency int
		result  string
	}{
		{"bandwidth_fail", 100, common.Fail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := client.NewDiagnoseClient("")
			task := NewBandWidthTask(cli, 20)
			bwTask := task.(*BandWidthTask)
			patch := gomonkey.ApplyMethod(reflect.TypeOf(bwTask), "RecordData", func(_ *BandWidthTask, resp *http.Response) int {
				return 100
			})
			defer patch.Reset()

			bwTask.Run(context.Background())
			assert.Equal(t, bwTask.output.Result, tt.result)
		})
	}
}
