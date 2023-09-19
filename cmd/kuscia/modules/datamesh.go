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

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/datamesh/commands"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type dataMeshModule struct {
	conf         *config.DataMeshConfig
	kusciaClient kusciaclientset.Interface
}

func NewDataMesh(d *Dependencies) Module {
	conf := config.NewDefaultDataMeshConfig()

	rootCAFile := d.CAFile
	if rootCAFile != "" && conf.TLSConfig != nil {
		conf.TLSConfig.RootCAFile = rootCAFile
	}
	// set namespace
	conf.KubeNamespace = d.DomainID
	nlog.Infof("Datamesh namespace:%s.", d.DomainID)
	return &dataMeshModule{
		conf:         conf,
		kusciaClient: d.Clients.KusciaClient,
	}
}

func (m dataMeshModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf, m.kusciaClient)
}

func (m dataMeshModule) WaitReady(ctx context.Context) error {
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
			return fmt.Errorf("wait datamesh ready timeout")
		}
	}
}

func (m dataMeshModule) Name() string {
	return "datamesh"
}

func (m dataMeshModule) readyZ() bool {
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// init client tls config
	tlsConfig := m.conf.TLSConfig
	if tlsConfig != nil {
		clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCAFile, tlsConfig.ServerCertFile, tlsConfig.ServerKeyFile)
		if err != nil {
			nlog.Errorf("local tls config error: %v", err)
			return false
		}
		schema = constants.SchemaHTTPS
	}

	// check http server ready
	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, constants.LocalhostIP, m.conf.HTTPPort, constants.HealthAPI)
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

func RunDataMesh(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewDataMesh(conf)
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
		nlog.Info("datamesh is ready")
	}
	return m
}
