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

package modules

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/secretflow/kuscia/pkg/reporter/commands"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	reporterpb "github.com/secretflow/kuscia/proto/api/v1alpha1/reporter"

	"github.com/secretflow/kuscia/pkg/reporter/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
)

type reporterModule struct {
	moduleRuntimeBase
	conf *config.ReporterConfig
}

func NewReporter(d *ModuleRuntimeConfigs) (Module, error) {
	reporterConfig := config.NewDefaultReporterConfig(d.RootDir)

	reporterConfig.RunMode = d.RunMode
	reporterConfig.DomainID = d.DomainID
	reporterConfig.DomainKey = d.DomainKey
	reporterConfig.KubeClient = d.Clients.KubeClient
	reporterConfig.KusciaClient = d.Clients.KusciaClient

	nlog.Debugf("Kuscia reporter config is %+v", reporterConfig)

	return &reporterModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "reporter",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewFuncReadyZ(func(ctx context.Context) error {
				if !ReporterReadyZ(reporterConfig.HTTPPort) {
					return errors.New("kuscia reporter is not ready now")
				}
				return nil
			}),
		},
		conf: reporterConfig,
	}, nil
}

func (m *reporterModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf)
}

func ReporterReadyZ(httpPort int32) bool {
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// check http server ready
	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, constants.LocalhostIP, httpPort, constants.HealthAPI)
	body, err := json.Marshal(&reporterpb.ReportProgressRequest{})
	if err != nil {
		nlog.Errorf("marshal health request error: %v", err)
		return false
	}
	req, err := http.NewRequest(http.MethodPost, httpURL, bytes.NewReader(body))
	if err != nil {
		nlog.Errorf("invalid request error: %v", err)
		return false
	}
	req.Header.Set(constants.ContentTypeHeader, constants.HTTPDefaultContentType)
	resp, err := httpClient.Do(req)
	if err != nil {
		nlog.Errorf("send health request error: %v", err)
		return false
	}
	if resp == nil || resp.Body == nil {
		nlog.Error("resp must has body")
		return false
	}
	defer resp.Body.Close()
	healthResp := &reporterpb.CommonReportResponse{}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Errorf("read response body error: %v", err)
		return false
	}
	if err := json.Unmarshal(respBytes, healthResp); err != nil {
		nlog.Errorf("Unmarshal health response error: %v", err)
		return false
	}
	if healthResp.Status.Code != int32(errorcode.ErrorCode_SUCCESS) {
		return false
	}
	nlog.Infof("http server is ready")

	return true
}
