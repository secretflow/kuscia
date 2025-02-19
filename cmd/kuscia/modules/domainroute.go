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
	"crypto/rsa"
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

func NewDomainRoute(i *ModuleRuntimeConfigs) (Module, error) {
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
			// save master pubkey for rsa token gen method
			nlog.Debugf("get peer master pubkey response: %s", response.MasterPubkey)
			pubkey, err := getPubkeyForToken(response.MasterPubkey, i.RunMode)
			if err != nil {
				nlog.Warnf("get peer response pubkey failed, err: %v, handshake can only use UID-RSA token", err)
				return
			}
			i.Master.MasterPubkey = pubkey
		},
	}, nil
}

func (d *domainRouteModule) Run(ctx context.Context) error {
	return commands.Run(ctx, d.conf, d.clients, d.afterRegisterHook)
}

func (d *domainRouteModule) WaitReady(ctx context.Context) error {
	return WaitChannelReady(ctx, commands.ReadyChan, 60*time.Second)
}

func (d *domainRouteModule) Name() string {
	return "domainroute"
}

func getPubkeyForToken(pubkey string, runmode common.RunModeType) (*rsa.PublicKey, error) {
	if pubkey == "" && runmode == common.RunModeLite {
		// for lite, but pubkey is empty, we can only use UID
		err := fmt.Errorf("query peer master pubkey failed, pubkey is empty")
		return nil, err
	}
	if runmode == common.RunModeLite {
		// for lite, try to decode master pubkey
		masterDer, decodeErr := base64.StdEncoding.DecodeString(pubkey)
		if decodeErr != nil {
			return nil, decodeErr
		}
		pubKey, parseErr := tlsutils.ParseRSAPublicKey(masterDer)
		if parseErr != nil {
			return nil, parseErr
		}
		return pubKey, nil
	}
	// otherwise, not need to acquire master pubkey
	return nil, nil
}
