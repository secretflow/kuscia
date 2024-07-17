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
	"sync/atomic"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/commands"
	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

const (
	serverCertsCommonName        = "ConfManager"
	defaultServerCertsSanDNSName = "confmanager"
)

type confManagerModule struct {
	conf *config.ConfManagerConfig
}

func NewConfManager(ctx context.Context, d *Dependencies) (Module, error) {
	// overwrite config
	conf := config.NewDefaultConfManagerConfig()
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
	if !d.ConfManager.EnableConfAuth {
		conf.EnableConfAuth = d.ConfManager.EnableConfAuth
	}
	if d.ConfManager.IsMaster {
		conf.IsMaster = d.ConfManager.IsMaster
	}
	if d.ConfManager.Backend != "" {
		conf.Backend = d.ConfManager.Backend
	}
	conf.DomainID = d.DomainID
	conf.DomainKey = d.DomainKey
	conf.TLS.RootCA = d.CACert
	conf.TLS.RootCAKey = d.CAKey
	switch d.RunMode {
	case common.RunModeLite:
		conf.DomainCertValue = &d.DomainCertByMasterValue
	case common.RunModeAutonomy:
		conf.DomainCertValue = &atomic.Value{}
		conf.DomainCertValue.Store(d.DomainCert)
	}
	secretBackend := findSecretBackend(d.SecretBackendHolder, conf.Backend)
	if secretBackend == nil {
		return nil, fmt.Errorf("failed to find secret backend %s for cm", conf.Backend)
	}
	conf.BackendDriver = secretBackend

	nlog.Debugf("Conf manager config is %+v", conf)

	if err := conf.TLS.GenerateServerKeyCerts(serverCertsCommonName, nil, []string{defaultServerCertsSanDNSName}); err != nil {
		return nil, err
	}

	// init service holder
	if err := service.InitServiceHolder(conf); err != nil {
		return nil, fmt.Errorf("init service holder failed: %v", err.Error())
	}

	return &confManagerModule{
		conf: conf,
	}, nil
}

func findSecretBackend(s *secretbackend.Holder, name string) secretbackend.SecretDriver {
	return s.Get(name)
}

func (m confManagerModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf)
}

func (m confManagerModule) WaitReady(ctx context.Context) error {
	timeoutTicker := time.NewTicker(30 * time.Second)
	defer timeoutTicker.Stop()
	checkTicker := time.NewTicker(1 * time.Second)
	defer checkTicker.Stop()
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
	tlsConfig := m.conf.TLS
	clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCA, tlsConfig.ServerCert, tlsConfig.ServerKey)
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
	nlog.Infof("http/https server is ready")
	return true
}

func RunConfManagerWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := NewShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "confmanager",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunConfManager(runCtx, cancel, conf, shutdownEntry)
}

func RunConfManager(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m, err := NewConfManager(ctx, conf)
	if err != nil {
		nlog.Error(err)
		cancel()
		return m
	}
	go func() {
		defer func() {
			if shutdownEntry != nil {
				shutdownEntry.RunShutdown()
			}
		}()
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Fatalf("ConfManager wait ready failed: %v", err)
	}
	nlog.Info("ConfManager is ready")
	return m
}
