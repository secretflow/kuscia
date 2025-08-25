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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
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
	svc = NewService(nil)
	m.Run()
}

type MockUtils struct {
	mock.Mock
}

func (m *MockUtils) GetLogAnalysisResult(taskID, logType string, parseTime *time.Time) ([]*diagnose.EnvoyLogInfo, error) {
	args := m.Called(taskID, logType, parseTime)
	return args.Get(0).([]*diagnose.EnvoyLogInfo), args.Error(1)
}

func (m *MockUtils) CSTTimeCovertToUTC(t time.Time) time.Time {
	args := m.Called(t)
	return args.Get(0).(time.Time)
}

func TestGetEnvoyLog_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := &diagnose.EnvoyLogRequest{
		TaskId:     "task_123",
		CreateTime: "20060102-15",
		DomainId:   "domain_1",
	}

	reqBody, _ := proto.Marshal(req)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/diagnose/envoy-log", bytes.NewReader(reqBody))
	svc = NewService(nil)
	svc.GetEnvoyLog(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetEnvoyLog_Failure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := &diagnose.EnvoyLogRequest{
		TaskId:     "task_123",
		CreateTime: "2022-01-02 15:04:05",
		DomainId:   "domain_1",
	}

	reqBody, _ := proto.Marshal(req)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/diagnose/envoy-log", bytes.NewReader(reqBody))
	svc = NewService(nil)
	svc.GetEnvoyLog(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
