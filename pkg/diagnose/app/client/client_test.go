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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/diagnose/app/server"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

var s *server.HTTPServerBean
var cli *Client

func TestMain(m *testing.M) {
	s = server.NewHTTPServerBean(nil)
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

func mockHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/diagnose/v1/network/envoy/log/task" {
		response := &diagnose.EnvoyLogInfoResponse{
			Status:   &v1alpha1.Status{Code: http.StatusOK},
			DomainId: "domain-b",
			EnvoyInfoList: []*diagnose.EnvoyLogInfo{
				{
					Type: common.InternalTypeLog,
					EnvoyLogList: []*diagnose.EnvoyLog{
						&diagnose.EnvoyLog{
							Ip:            "127.0.0.1",
							Timestamp:     "2023-01-01T00:00:00Z",
							NodeName:      "alice",
							ServiceName:   "secretflow-task-20250530143614-single-psi-0-spu.bob.svc",
							InterfaceAddr: "0.0.0.0:8080",
							HttpMethod:    "GET",
							TraceId:       "12345",
							StatusCode:    "500",
							ContentLength: "1024",
							RequestTime:   "0.1s",
						},
						&diagnose.EnvoyLog{
							Ip:            "127.0.0.1",
							Timestamp:     "2023-01-01T00:00:00Z",
							NodeName:      "alice",
							ServiceName:   "secretflow-task-20250530143614-single-psi-0-spu.bob.svc",
							InterfaceAddr: "0.0.0.0:8080",
							HttpMethod:    "GET",
							TraceId:       "12345",
							StatusCode:    "502",
							ContentLength: "1024",
							RequestTime:   "0.1s",
						},
					},
				},
				{
					Type: common.ExternalTypeLog,
					EnvoyLogList: []*diagnose.EnvoyLog{
						&diagnose.EnvoyLog{
							Ip:            "127.0.0.1",
							Timestamp:     "2023-01-01T00:00:00Z",
							NodeName:      "alice",
							ServiceName:   "secretflow-task-20250530143614-single-psi-0-spu.bob.svc",
							InterfaceAddr: "0.0.0.0:8080",
							HttpMethod:    "GET",
							TraceId:       "12345",
							StatusCode:    "501",
							ContentLength: "1024",
							RequestTime:   "0.1s",
						},
						&diagnose.EnvoyLog{
							Ip:            "127.0.0.1",
							Timestamp:     "2023-01-01T00:00:00Z",
							NodeName:      "alice",
							ServiceName:   "secretflow-task-20250530143614-single-psi-0-spu.bob.svc",
							InterfaceAddr: "0.0.0.0:8080",
							HttpMethod:    "GET",
							TraceId:       "12345",
							StatusCode:    "503",
							ContentLength: "1024",
							RequestTime:   "0.1s",
						},
					},
				},
			},
		}
		data, err := proto.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to marshal protobuf response", http.StatusInternalServerError)
			return
		}

		// set HTTPHeader application/x-protobuf
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)

		w.Write(data)
	}
}

func TestEnvoyLog(t *testing.T) {
	// mock handler
	httpServer := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer httpServer.Close()
	cli = NewDiagnoseClient(httpServer.URL)
	response, err := cli.EnvoyLog(context.Background(), &diagnose.EnvoyLogRequest{TaskId: "secretflow-task-20250530143614-single", CreateTime: time.Time{}.Format(common.LogTimeFormat)})
	assert.NoError(t, err)
	assert.Equal(t, int32(200), response.Status.Code)
}
