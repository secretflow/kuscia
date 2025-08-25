// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netstat

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	commonl "github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

func TestNewTaskAnalysis(t *testing.T) {
	// test NewTaskAnalysis
	taskID := "test-task"
	domainID := "test-domain"
	ta := NewTaskAnalysis(taskID, domainID, "", "", common.RunModeAutonomy)

	assert.Equal(t, taskID, ta.taskID)
	assert.Equal(t, domainID, ta.domainID)
}

func makeTestKusciaTask(phase kusciaapisv1alpha1.KusciaTaskPhase, now time.Time) *kusciaapisv1alpha1.KusciaTask {
	return &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "kusciatask-001",
			Namespace:         common.KusciaCrossDomain,
			CreationTimestamp: metav1.NewTime(now),
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			TaskInputConfig: "task input config",
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "domain-a",
					AppImageRef: "test-image-1",
					Role:        "server",
				},
				{
					DomainID:    "domain-b",
					AppImageRef: "test-image-1",
					Role:        "client",
				},
			},
		},
		Status: kusciaapisv1alpha1.KusciaTaskStatus{Phase: phase},
	}
}

func mockHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/diagnose/v1/network/envoy/log/task" {
		response := &diagnose.EnvoyLogInfoResponse{
			Status:   &v1alpha1.Status{Code: http.StatusOK},
			DomainId: "domain-b",
			EnvoyInfoList: []*diagnose.EnvoyLogInfo{
				{
					Type: commonl.InternalTypeLog,
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
					Type: commonl.ExternalTypeLog,
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

func mockLocalLog(t *testing.T) string {
	tempDir := t.TempDir()
	os.Setenv("KUSCIA_HOME", tempDir)
	defer os.Unsetenv("KUSCIA_HOME")

	// mkdir envoy log dir
	logDir := filepath.Join(tempDir, "var/logs/envoy")
	err := os.MkdirAll(logDir, 0755)
	assert.NoError(t, err)

	// mkdir internal log dir
	internalLog := filepath.Join(logDir, "internal.log")
	err = os.WriteFile(internalLog, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 503 - 1424 283 206 0 105 101 - -`), 0644)
	assert.NoError(t, err)

	// mkdir internal log  dir
	internalLogDate := filepath.Join(logDir, "internal.log-20230101-12")
	err = os.WriteFile(internalLogDate, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 504 - 1424 283 206 0 105 101 - -`), 0644)
	assert.NoError(t, err)

	// mkdir external log dir
	externalLog := filepath.Join(logDir, "external.log")
	err = os.WriteFile(externalLog, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 505 - 118783 33 202 - -`), 0644)
	assert.NoError(t, err)

	// mkdir external log  dir
	externalLogDate := filepath.Join(logDir, "external.log-20230101-12")
	err = os.WriteFile(externalLogDate, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 506 - 118783 33 202 - -`), 0644)
	assert.NoError(t, err)
	return tempDir

}

func TestAnalysis(t *testing.T) {
	// mock local log
	rootDir := mockLocalLog(t)
	// mock handler
	server := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer server.Close()
	taskID := "kusciatask-001"
	domainID := "domain-a"

	ta := NewTaskAnalysis(taskID, domainID, rootDir, server.URL, common.RunModeAutonomy)
	_, err := ta.Analysis()
	assert.Error(t, err)
}
