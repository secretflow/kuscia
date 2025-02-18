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

package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/secretflow/kuscia/pkg/diagnose/app/server"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"github.com/stretchr/testify/assert"
)

var s *server.HTTPServerBean
var cli *Client

func TestMain(m *testing.M) {
	s = server.NewHTTPServerBean()
	go s.Run(context.Background())
	cli = NewDiagnoseClient("localhost:8095")
	m.Run()
}

func TestSubmitReport(t *testing.T) {
	testcase := []*diagnose.MetricItem{
		{
			Name:          "Latency",
			DetectedValue: "10ms",
			Threshold:     "100ms",
			Result:        "[PASS]",
			Information:   "",
		},
	}
	resp, err := cli.SubmitReport(context.Background(), &diagnose.SubmitReportRequest{
		Items: testcase,
	})
	assert.Nil(t, err)
	assert.EqualValues(t, 200, resp.Status.Code)
	assert.Equal(t, true, s.Service.RecReportDone)
}

func TestHealthy(t *testing.T) {
	resp, err := cli.Healthy(context.Background())
	assert.Nil(t, err)
	assert.EqualValues(t, 200, resp.Status.Code)
}

func TestMock(t *testing.T) {
	testcase := 1024
	resp, err := cli.Mock(context.Background(), &diagnose.MockRequest{
		RespBodySize: int64(testcase),
	})
	assert.Nil(t, err)
	assert.EqualValues(t, 200, resp.Status.Code)
	assert.Equal(t, testcase, len(resp.Data))
}

func TestMockChunk(t *testing.T) {
	testcase := 10240
	chunkSize := 1024
	url := fmt.Sprintf("http://%v/%v/%v", cli.HostName, common.DiagnoseNetworkGroup, common.DiagnoseMockPath)

	resp, err := cli.MockChunk(&diagnose.MockRequest{
		EnableChunked:   true,
		ChunkedSize:     int64(chunkSize),
		ChunkedInterval: 100,
		RespBodySize:    10240,
	}, url)
	assert.Nil(t, err)

	size := 0
	buffer := make([]byte, testcase)
	for {
		recvSize, err := resp.Body.Read(buffer)
		if err != nil {
			nlog.Infof("Read End: %v\n", err)
			break
		}
		assert.Equal(t, chunkSize, recvSize)
		size += recvSize
	}
}
