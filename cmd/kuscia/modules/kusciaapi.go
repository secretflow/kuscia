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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.ame
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
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/commands"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	apiutils "github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	webconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

const (
	kusciaAPISanDNSName            = "kusciaapi"
	kusciaAPIInterceptorLoggerPath = "var/logs/kusciaapi.log"
	dateMeshInterceptorLoggerPath  = "var/logs/datamesh.log"
)

type kusciaAPIModule struct {
	moduleRuntimeBase
	conf         *config.KusciaAPIConfig
	kusciaClient kusciaclientset.Interface
	kubeClient   kubernetes.Interface
}

func NewKusciaAPI(d *ModuleRuntimeConfigs) (Module, error) {
	kusciaAPIConfig := d.KusciaAPI
	if d.RunMode != common.RunModeMaster {
		kusciaAPIConfig.Initiator = d.DomainID
	}
	kusciaAPIConfig.RootCAKey = d.CAKey
	kusciaAPIConfig.RootCA = d.CACert
	kusciaAPIConfig.DomainKey = d.DomainKey
	kusciaAPIConfig.TLS.RootCA = d.CACert
	kusciaAPIConfig.TLS.RootCAKey = d.CAKey
	kusciaAPIConfig.TLS.CommonName = "KusciaAPI"
	kusciaAPIConfig.RunMode = d.RunMode
	kusciaAPIConfig.DomainCertValue = &d.DomainCertByMasterValue
	kusciaAPIConfig.DomainID = d.DomainID
	kusciaAPIConfig.Protocol = d.Protocol
	kusciaAPIConfig.StdoutPath = d.Agent.StdoutPath
	kusciaAPIConfig.NodeName = d.Agent.Node.NodeName

	protocol := kusciaAPIConfig.Protocol
	if protocol == "" {
		kusciaAPIConfig.Protocol = common.MTLS
		protocol = common.MTLS
	} else if protocol == common.NOTLS {
		kusciaAPIConfig.TLS = nil
		kusciaAPIConfig.Token = nil
	}

	if kusciaAPIConfig.TLS != nil {
		if err := kusciaAPIConfig.TLS.LoadFromDataOrFile(nil, []string{kusciaAPISanDNSName}); err != nil {
			return nil, err
		}
	}

	if kusciaAPIConfig.Token != nil {
		tokenFile := kusciaAPIConfig.Token.TokenFile
		if tokenFile != "" && !paths.CheckFileExist(tokenFile) {
			tokenData, err := tlsutils.SignWithRSA(kusciaAPIConfig.DomainKey, kusciaAPIConfig.DomainID)
			if err != nil {
				nlog.Errorf("Generate token file error: %v", err.Error())
				return nil, err
			}
			if err := os.WriteFile(tokenFile, []byte(tokenData[:32]), 0644); err != nil {
				nlog.Errorf("Write token file error: %v", err.Error())
				return nil, err
			}
		}
	}
	interceptorLogger, err := initInterceptorLogger(d.KusciaConfig, kusciaAPIInterceptorLoggerPath)
	if err != nil {
		nlog.Errorf("Init Kuscia API interceptor logger failed: %v", err)
		return nil, err
	}

	kusciaAPIConfig.InterceptorLog = interceptorLogger
	nlog.Debugf("Kuscia api config is %+v", kusciaAPIConfig)

	return &kusciaAPIModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "kusciapi",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewFuncReadyZ(func(ctx context.Context) error {
				if !KusciaAPIReadyZ(kusciaAPIConfig.TLS, kusciaAPIConfig.HTTPPort, kusciaAPIConfig.GRPCPort, kusciaAPIConfig.Protocol, kusciaAPIConfig.Token) {
					return errors.New("kuscia is not ready now")
				}
				return nil
			}),
		},
		conf:         kusciaAPIConfig,
		kusciaClient: d.Clients.KusciaClient,
		kubeClient:   d.Clients.KubeClient,
	}, nil
}

func initInterceptorLogger(kusciaConf confloader.KusciaConfig, logPath string) (*nlog.NLog, error) {
	logConfig := initLoggerConfig(kusciaConf, logPath)
	logWriter, err := zlogwriter.New(logConfig)
	if err != nil {
		return nil, err
	}
	return nlog.NewNLog(nlog.SetWriter(logWriter), nlog.SetFormatter(nlog.NewGinLogFormatter())), nil
}

func (m *kusciaAPIModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf, m.kusciaClient, m.kubeClient)
}

func KusciaAPIReadyZ(tlsConfig *webconfig.TLSServerConfig, httpPort, grpcPort int32, protocol common.Protocol, tokenConfig *config.TokenConfig) bool {
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// init client tls config
	if tlsConfig != nil {
		if protocol == common.TLS {
			clientTLSConfig, err = tlsutils.BuildClientSimpleTLSConfig(tlsConfig.ServerCert)
		} else {
			clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCA, tlsConfig.ServerCert, tlsConfig.ServerKey)
		}
		if err != nil {
			nlog.Errorf("local tls config error: %v", err)
			return false
		}
		schema = constants.SchemaHTTPS
	}

	// token auth
	var token string
	var tokenAuth bool
	if tokenConfig != nil {
		token, err = apiutils.ReadToken(*tokenConfig)
		if err != nil {
			nlog.Error(err.Error())
			return false
		}
		tokenAuth = true
	}

	// check http server ready
	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, constants.LocalhostIP, httpPort, constants.HealthAPI)
	body, err := json.Marshal(&kusciaapi.HealthRequest{})
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
	if tokenAuth {
		req.Header.Set(constants.TokenHeader, token)
	}
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
	healthResp := &kusciaapi.HealthResponse{}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Errorf("read response body error: %v", err)
		return false
	}
	if err := json.Unmarshal(respBytes, healthResp); err != nil {
		nlog.Errorf("Unmarshal health response error: %v", err)
		return false
	}
	if !healthResp.Data.Ready {
		return false
	}
	nlog.Infof("http server is ready")

	// check grpc server ready
	dialOpts := make([]grpc.DialOption, 0)
	if clientTLSConfig != nil {
		creds := credentials.NewTLS(clientTLSConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	// add token interceptor
	if tokenAuth {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(interceptor.GrpcClientTokenInterceptor(token)))
	}

	grpcAddr := fmt.Sprintf("%s:%d", constants.LocalhostIP, grpcPort)
	grpcConn, err := grpc.Dial(grpcAddr, dialOpts...)
	if err != nil {
		nlog.Fatalf("did not connect: %v", err)
	}
	defer grpcConn.Close()
	grpcClient := kusciaapi.NewHealthServiceClient(grpcConn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := grpcClient.HealthZ(ctx, &kusciaapi.HealthRequest{})
	if err != nil {
		return false
	}
	return res.Data.Ready
}
