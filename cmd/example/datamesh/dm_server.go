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

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/commands"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/flight/example"
	kusciaapiconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	binaryTestData        = "binary"
	primitivesTestData    = "primitives"
	mockDataProxyEndpoint = "localhost:8086"
	dataMeshHost          = "localhost"
	mockDomain            = "mock-domain"
)

type opts struct {
	dataProxyEndpoint string
	startClient       bool
	startDataMesh     bool
	enableDataMeshTLS bool
	testDataType      string
	outputCSVFilePath string
	logCfg            *nlog.LogConfig
}

func (o *opts) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.dataProxyEndpoint, "dataProxyEndpoint", mockDataProxyEndpoint,
		"data proxy endpoint")
	fs.BoolVar(&o.startClient, "startClient", true, "startClient")
	fs.BoolVar(&o.startDataMesh, "startDataMesh", true, "startDataMesh")
	fs.BoolVar(&o.enableDataMeshTLS, "enableDataMeshTLS", false, "enableDataMeshTLS")
	fs.StringVar(&o.testDataType, "testDataType", primitivesTestData, "binary or primitives,"+
		"binary refers to schema [{binary,nullable}], primitives refers to [{bool,nullable},{int64,nullable},{float64,"+
		"nullable}]")
	fs.StringVar(&o.outputCSVFilePath, "outputCSVFilePath", "./a.csv",
		"outputCSVFilePath")
	o.logCfg = zlogwriter.InstallPFlags(fs)
}

func main() {
	o := &opts{}
	rootCmd := newCommand(context.Background(), o)
	o.AddFlags(rootCmd.Flags())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newCommand(ctx context.Context, o *opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "flightMetaServer",
		Long:         "Mock flightMetaServer",
		Version:      meta.KusciaVersionString(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			zLogWriter, err := zlogwriter.New(o.logCfg)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zLogWriter))

			certsConfig, err := createClientCertificate()
			if err != nil {
				nlog.Fatalf("create cert file fail :%v", err)
			}

			conf := config.NewDefaultDataMeshConfig()
			conf.ExternalDataProxyList = []config.ExternalDataProxyConfig{
				{
					Endpoint: o.dataProxyEndpoint,
				},
			}
			conf.KubeNamespace = mockDomain
			conf.KusciaClient = kusciafake.NewSimpleClientset()
			conf.DisableTLS = !o.enableDataMeshTLS

			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				nlog.Fatal("generate domainKey fail")
				return nil
			}
			conf.DomainKey = privateKey

			if conf.TLS.ServerKey, err = tls.ParseKey([]byte{}, certsConfig.serverKeyFile); err != nil {
				return err
			}
			if conf.TLS.ServerCert, err = tls.ParseCert([]byte{}, certsConfig.serverCertFile); err != nil {
				return err
			}
			if conf.TLS.RootCA, err = tls.ParseCert([]byte{}, certsConfig.caFile); err != nil {
				return err
			}

			if conf.DomainKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
				nlog.Errorf("generate DomainKey fail:%s", err.Error())
				return nil
			}
			runCtx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
			}()

			wg := sync.WaitGroup{}
			if o.dataProxyEndpoint == mockDataProxyEndpoint {
				wg.Add(1)
				defer wg.Done()
				go startMockDataProxy(cancel, o)
			}

			if o.startClient {
				wg.Add(1)
				defer wg.Done()
				go startClient(cancel, o, certsConfig, conf)
			}

			go func() {
				wg.Add(1)
				defer wg.Done()
				if o.startDataMesh {
					if err := commands.Run(runCtx, conf, conf.KusciaClient); err != nil {
						cancel()
					}
				} else {
					time.Sleep(time.Minute * 7200)
					nlog.Infof("mock example exit by timeout")
				}

			}()
			wg.Wait()
			<-runCtx.Done()
			return nil
		},
	}
	return cmd
}

func startMockDataProxy(cancel context.CancelFunc, o *opts) {
	fmt.Println(o.dataProxyEndpoint)
	dp := example.NewMockDataProxy(o.dataProxyEndpoint)
	if err := dp.Start(); err != nil {
		cancel()
	}
}

func startClient(cancel context.CancelFunc, o *opts, certConfig *CertsConfig, dmConfig *config.DataMeshConfig) {
	if !o.enableDataMeshTLS {
		certConfig = nil
	}

	kusciaAPIConfig := &kusciaapiconfig.KusciaAPIConfig{
		DomainKey:    dmConfig.DomainKey,
		KusciaClient: dmConfig.KusciaClient,
		RunMode:      common.RunModeLite,
		Initiator:    dmConfig.KubeNamespace,
		DomainID:     dmConfig.KubeNamespace,
	}
	client := &MockFlightClient{
		testDataType:      o.testDataType,
		outputCSVFilePath: o.outputCSVFilePath,
		datasourceSvc:     service.NewDomainDataSourceService(kusciaAPIConfig, makeMemConfigurationService()),
	}

	// wait a while to wait data proxy ready to serve
	time.Sleep(time.Second * 10)
	if err := client.start(certConfig); err != nil {
		cancel()
	}
}

func makeMemConfigurationService() cmservice.IConfigurationService {
	backend, _ := secretbackend.NewSecretBackendWith("mem", map[string]any{})
	configurationService, _ := cmservice.NewConfigurationService(
		backend, false,
	)
	return configurationService
}
