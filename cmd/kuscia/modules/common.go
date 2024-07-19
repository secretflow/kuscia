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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/process"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	initProcessOOMScore  = -999
	kusciaOOMScore       = -900
	k3sOOMScore          = -800
	containerdOOMScore   = -700
	envoyOOMScore        = -700
	nodeExporterOOMScore = -600
)

func SetKusciaOOMScore() {
	if err := process.SetOOMScore(1, initProcessOOMScore); err != nil {
		nlog.Warnf("Set init process oom_score_adj failed, %v, skip setting it", err)
	}

	if err := process.SetOOMScore(os.Getpid(), kusciaOOMScore); err != nil {
		nlog.Warnf("Set kuscia controllers process oom_score_adj failed, %v, skip setting it", err)
	}
}

func KusciaServiceReadyZ(tlsConfig *config.TLSServerConfig, port int32) error {
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// init client tls config
	clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCA, tlsConfig.ServerCert, tlsConfig.ServerKey)
	if err != nil {
		return fmt.Errorf("local tls config error: %v", err)
	}
	schema = constants.SchemaHTTPS

	// check http server ready
	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, constants.LocalhostIP, port, constants.HealthAPI)
	body, err := json.Marshal(&kusciaapi.HealthRequest{})
	if err != nil {
		return fmt.Errorf("marshal health request error: %v", err)
	}
	resp, err := httpClient.Post(httpURL, constants.HTTPDefaultContentType, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send health request error: %v", err)
	}
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("resp must has body")

	}
	defer resp.Body.Close()
	healthResp := &kusciaapi.HealthResponse{}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body error: %v", err)
	}
	if err = json.Unmarshal(respBytes, healthResp); err != nil {
		return fmt.Errorf("unmarshal health response error: %v", err)
	}

	if healthResp.Data == nil || !healthResp.Data.Ready {
		return fmt.Errorf("input response data is nil")
	}

	nlog.Infof("http server is ready")
	return nil
}
