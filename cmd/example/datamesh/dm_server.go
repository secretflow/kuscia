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
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/driver"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/commands"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	kusciaapiconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const (
	binaryTestData        = "binary"
	primitivesTestData    = "primitives"
	mockDataProxyEndpoint = "localhost:8086"
	dataMeshHost          = "localhost"
	mockDomain            = "mock-domain"
)

type opts struct {
	listenAddr        string
	dataProxyEndpoint string
	startClient       bool
	startDataMesh     bool
	enableDataMeshTLS bool
	testDataType      string
	outputCSVFilePath string
	logCfg            *nlog.LogConfig
	// oss data source
	ossDataSourceID string
	ossEndpoint     string
	ossAccessKey    string
	ossAccessSecret string
	ossBucket       string
	ossPrefix       string
	ossType         string
}

func (o *opts) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.listenAddr, "listenAddr", "0.0.0.0", "datamesh bind address")
	fs.StringVar(&o.dataProxyEndpoint, "dataProxyEndpoint", mockDataProxyEndpoint,
		"data proxy endpoint")
	fs.BoolVar(&o.startClient, "startClient", false, "startTestClient")
	fs.BoolVar(&o.startDataMesh, "startDataMesh", false, "startDataMeshServer")
	fs.BoolVar(&o.enableDataMeshTLS, "enableDataMeshTLS", false, "enableDataMeshTLS")
	fs.StringVar(&o.testDataType, "testDataType", primitivesTestData, "binary or primitives,"+
		"binary refers to schema [{binary,nullable}], primitives refers to [{bool,nullable},{int64,nullable},{float64,"+
		"nullable}]")
	fs.StringVar(&o.outputCSVFilePath, "outputCSVFilePath", "./a.csv", "outputCSVFilePath")

	fs.StringVar(&o.ossDataSourceID, "ossDataSource", "", "oss data source id")
	fs.StringVar(&o.ossEndpoint, "ossEndpoint", "127.0.0.1:9000", "oss endpoint")
	fs.StringVar(&o.ossAccessKey, "ossAccessKey", "", "oss access key")
	fs.StringVar(&o.ossAccessSecret, "ossAccessSecret", "", "oss access secret")
	fs.StringVar(&o.ossBucket, "ossBucket", "", "oss bucket")
	fs.StringVar(&o.ossPrefix, "ossPrefix", "/", "oss path prefix")
	fs.StringVar(&o.ossType, "ossType", "oss", "minio/oss")

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
				return fmt.Errorf("create cert file fail :%v", err)

			}
			conf := config.NewDefaultDataMeshConfig()
			conf.KubeNamespace = mockDomain
			conf.KusciaClient = kusciafake.NewSimpleClientset()
			conf.KubeClient = kubefake.NewSimpleClientset()
			conf.ListenAddr = o.listenAddr

			if err := loadConfigFromCmdEnv(conf, certsConfig, o); err != nil {
				nlog.Fatal(err.Error())
				return nil
			}

			if err := setupOssDataSource(ctx, conf, o); err != nil {
				nlog.Fatal(err.Error())
				return nil
			}

			paths.EnsureDirectory(common.DefaultDomainDataSourceLocalFSPath, true)

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

			wg.Add(1)
			go func() {
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
	time.Sleep(1000 * time.Second)
	cancel()
}

func startClient(cancel context.CancelFunc, o *opts, certConfig *CertsConfig, dmConfig *config.DataMeshConfig) {
	if !o.enableDataMeshTLS {
		certConfig = nil
	}

	kusciaAPIConfig := &kusciaapiconfig.KusciaAPIConfig{
		DomainKey:    dmConfig.DomainKey,
		KusciaClient: dmConfig.KusciaClient,
		KubeClient:   dmConfig.KubeClient,
		RunMode:      common.RunModeLite,
		Initiator:    dmConfig.KubeNamespace,
		DomainID:     dmConfig.KubeNamespace,
	}

	client := &MockFlightClient{
		testDataType:      o.testDataType,
		outputCSVFilePath: o.outputCSVFilePath,
		datasourceSvc:     service.NewDomainDataSourceService(kusciaAPIConfig, makeConfigurationService(kusciaAPIConfig)),
	}

	// wait a while to wait data proxy ready to serve
	time.Sleep(time.Second * 10)
	if err := client.start(certConfig); err != nil {
		cancel()
	}
}

func makeConfigurationService(kusciaAPIConfig *kusciaapiconfig.KusciaAPIConfig) cmservice.IConfigService {
	configurationService, _ := cmservice.NewConfigService(
		context.Background(),
		&cmservice.ConfigServiceConfig{
			DomainID:   kusciaAPIConfig.DomainID,
			DomainKey:  kusciaAPIConfig.DomainKey,
			Driver:     driver.CRDDriverType,
			KubeClient: kusciaAPIConfig.KubeClient,
		},
	)
	return configurationService
}

func loadConfigFromCmdEnv(conf *config.DataMeshConfig, certsConfig *CertsConfig, o *opts) error {
	conf.DisableTLS = !o.enableDataMeshTLS
	conf.DataProxyList = []config.DataProxyConfig{
		{
			Endpoint: o.dataProxyEndpoint,
		},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate domainKey fail")
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
		return fmt.Errorf("generate DomainKey fail:%s", err.Error())
	}
	return nil
}

func setupOssDataSource(ctx context.Context, conf *config.DataMeshConfig, o *opts) error {
	if o.ossDataSourceID != "" && o.ossBucket != "" {
		ossConfig, _ := json.Marshal(&datamesh.DataSourceInfo{
			Oss: &datamesh.OssDataSourceInfo{
				Endpoint:        o.ossEndpoint,
				AccessKeyId:     o.ossAccessKey,
				AccessKeySecret: o.ossAccessSecret,
				Bucket:          o.ossBucket,
				Prefix:          o.ossPrefix,
				StorageType:     o.ossType,
				Version:         "s3v4",
				Virtualhost:     false,
			}})

		strConfig, _ := tls.EncryptOAEP(&conf.DomainKey.PublicKey, ossConfig)

		_, err := conf.KusciaClient.KusciaV1alpha1().DomainDataSources(mockDomain).Create(ctx, &v1alpha1.DomainDataSource{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.ossDataSourceID,
			},
			Spec: v1alpha1.DomainDataSourceSpec{
				URI:            o.ossEndpoint,
				Type:           common.DomainDataSourceTypeOSS,
				Name:           o.ossDataSourceID,
				AccessDirectly: false,
				Data:           map[string]string{"encryptedInfo": strConfig},
			},
		}, v1.CreateOptions{})

		return err
	}
	return nil
}
