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
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/gateway/commands"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/controller"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/handshake"
)

type domainRouteModule struct {
	conf              *config.GatewayConfig
	clients           *kubeconfig.KubeClients
	afterRegisterHook controller.AfterRegisterDomainHook
}

func NewDomainRoute(i *Dependencies) Module {
	conf := config.DefaultStaticGatewayConfig()
	conf.RootDir = i.RootDir
	conf.ConfBasedir = filepath.Join(i.RootDir, common.ConfPrefix, "domainroute")
	conf.DomainID = i.DomainID
	conf.DomainKey = i.DomainKey
	conf.MasterConfig = &i.Master
	conf.CsrData = i.DomainRoute.DomainCsrData
	conf.CACert = i.CACert
	conf.CAKey = i.CAKey

	externalTLS := conf.ExternalTLS
	if i.DomainRoute.ExternalTLS != nil {
		externalTLS = i.DomainRoute.ExternalTLS
	}

	protocol := i.Protocol
	switch protocol {
	case common.NOTLS:
		externalTLS = nil
	case common.TLS, common.MTLS:
		externalTLS = &kusciaconfig.TLSConfig{
			EnableTLS: true,
		}
	}

	if externalTLS != nil && externalTLS.EnableTLS {
		if externalTLS.KeyData == "" && externalTLS.CertData == "" {
			var err error
			externalTLS.KeyData, externalTLS.CertData, err = tlsutils.GenerateKeyCertPairData(i.CAKey, i.CACert, fmt.Sprintf("%s_ENVOY_EXTERNAL", conf.DomainID))
			if err != nil {
				nlog.Fatalf("Generate external keyCert pair error:%v", err.Error())
			}
		}
	}
	conf.ExternalTLS = externalTLS

	if i.TransportPort > 0 {
		conf.TransportConfig = &kusciaconfig.ServiceConfig{
			Endpoint: fmt.Sprintf("http://127.0.0.1:%d", i.TransportPort),
		}
	}
	if i.InterConnSchedulerPort > 0 {
		conf.InterConnSchedulerConfig = &kusciaconfig.ServiceConfig{
			Endpoint: fmt.Sprintf("http://127.0.0.1:%d", i.InterConnSchedulerPort),
		}
	}

	return &domainRouteModule{
		conf:    conf,
		clients: i.Clients,
		afterRegisterHook: func(response *handshake.RegisterResponse) {
			if response.Cert == "" {
				return
			}
			certBytes, err := base64.StdEncoding.DecodeString(response.Cert)
			if err != nil {
				nlog.Warnf("decode domain cert base64 failed")
				return
			}
			domainCert, err := tlsutils.DecodeCert(certBytes)
			if err != nil {
				nlog.Warnf("decode domain cert failed")
			}
			i.DomainCertByMasterValue.Store(domainCert)
		},
	}
}

func (d *domainRouteModule) Run(ctx context.Context) error {
	return commands.Run(ctx, d.conf, d.clients, d.afterRegisterHook)
}

func (d *domainRouteModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	select {
	case <-commands.ReadyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return fmt.Errorf("wait domainroute ready timeout")
	}
}

func (d *domainRouteModule) Name() string {
	return "domainroute"
}

func RunDomainRoute(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewDomainRoute(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Errorf("domain route wait ready failed with error: %v", err)
		cancel()
	} else {
		nlog.Info("domainroute is ready")
	}

	return m
}
