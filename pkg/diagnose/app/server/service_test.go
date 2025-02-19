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

package server

import (
	"testing"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"github.com/stretchr/testify/assert"
)

var svc *DiagnoseService

func TestHealthy(t *testing.T) {
	resp := svc.healthy()
	assert.Equal(t, 200, int(resp.Status.Code))
}

func TestMock(t *testing.T) {
	size := 1000
	resp := svc.mock(&diagnose.MockRequest{
		RespBodySize: int64(size),
	})
	assert.Equal(t, 200, int(resp.Status.Code))
	assert.Equal(t, size, len(resp.Data))
}

func TestSubmitReport(t *testing.T) {
	items := []*diagnose.MetricItem{
		{
			Name:          "BANDWIDTH",
			DetectedValue: "1000",
			Threshold:     "100",
			Result:        "xxx",
			Information:   "xxx",
		},
	}
	resp := svc.submitReport(&diagnose.SubmitReportRequest{
		Items: items,
	})
	assert.Equal(t, 200, int(resp.Status.Code))
	assert.Equal(t, true, svc.RecReportDone)

}

func TestMain(m *testing.M) {
	svc = NewService()
	m.Run()
}
