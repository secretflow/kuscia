// Copyright 2023 Ant Group Co., Ltd.
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
	"fmt"
	"io"
	"time"

	"github.com/secretflow/kuscia/pkg/confmanager/commands"
	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type confManagerModule struct {
	conf *config.ConfManagerConfig
}

func NewConfManager(d *Dependencies) Module {
	// overwrite config
	conf := config.NewDefaultConfManagerConfig(d.RootDir)
	if d.ConfManager.HTTPPort != 0 {
		conf.HTTPPort = d.ConfManager.HTTPPort
	}
	if d.ConfManager.GRPCPort != 0 {
		conf.GRPCPort = d.ConfManager.GRPCPort
	}
	if d.ConfManager.ConnectTimeout != 0 {
		conf.ConnectTimeout = d.ConfManager.ConnectTimeout
	}
	if d.ConfManager.ReadTimeout != 0 {
		conf.ReadTimeout = d.ConfManager.ReadTimeout
	}
	if d.ConfManager.WriteTimeout != 0 {
		conf.WriteTimeout = d.ConfManager.WriteTimeout
	}
	if d.ConfManager.IdleTimeout != 0 {
		conf.IdleTimeout = d.ConfManager.IdleTimeout
	}
	if d.ConfManager.EnableConfAuth != false {
		conf.EnableConfAuth = d.ConfManager.EnableConfAuth
	}
	if d.ConfManager.TLSConfig != nil {
		conf.TLSConfig = d.ConfManager.TLSConfig
	}
	if d.ConfManager.SecretBackend != nil && d.ConfManager.SecretBackend.Driver != "" {
		conf.SecretBackend = d.ConfManager.SecretBackend
	}

	// set namespace
	nlog.Infof("ConfManager namespace:%s.", d.DomainID)
	return &confManagerModule{
		conf: conf,
	}
}

func (m confManagerModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf)
}

func (m confManagerModule) WaitReady(ctx context.Context) error {
	timeoutTicker := time.NewTicker(30 * time.Second)
	checkTicker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-checkTicker.C:
			if m.readyZ() {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTicker.C:
			return fmt.Errorf("wait confmanager ready timeout")
		}
	}
}

func (m confManagerModule) Name() string {
	return "confmanager"
}

func (m confManagerModule) readyZ() bool {
	var clientTLSConfig *tls.Config
	var err error
	// init client tls config
	tlsConfig := m.conf.TLSConfig
	clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCAFile, tlsConfig.ServerCertFile, tlsConfig.ServerKeyFile)
	if err != nil {
		nlog.Errorf("local tls config error: %v", err)
		return false
	}

	// check http server ready
	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", constants.SchemaHTTPS, constants.LocalhostIP, m.conf.HTTPPort, constants.HealthAPI)
	body, err := json.Marshal(&kusciaapi.HealthRequest{})
	if err != nil {
		nlog.Errorf("marshal health request error: %v", err)
		return false
	}
	resp, err := httpClient.Post(httpURL, constants.HTTPDefaultContentType, bytes.NewReader(body))
	if err != nil {
		nlog.Errorf("send health request error: %v", err)
		return false
	}
	if resp == nil || resp.Body == nil {
		nlog.Error("resp must has body")
		return false
	}
	defer resp.Body.Close()
	healthResp := &kusciaapi.HealthResponse{}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Errorf("read response body error: %v", err)
		return false
	}
	if err = json.Unmarshal(respBytes, healthResp); err != nil {
		nlog.Errorf("Unmarshal health response error: %v", err)
		return false
	}
	if healthResp.Data == nil || !healthResp.Data.Ready {
		return false
	}
	nlog.Infof("http server is ready")
	return true
}

func RunConfManager(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewConfManager(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Error(err)
		cancel()
	} else {
		nlog.Info("confmanager is ready")
	}
	return m
}
