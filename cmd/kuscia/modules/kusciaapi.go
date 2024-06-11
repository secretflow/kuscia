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
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

const (
	kusciaAPISanDNSName            = "kusciaapi"
	kusciaAPIInterceptorLoggerPath = "var/logs/kusciaapi.log"
)

type kusciaAPIModule struct {
	conf         *config.KusciaAPIConfig
	kusciaClient kusciaclientset.Interface
	kubeClient   kubernetes.Interface
}

func NewKusciaAPI(d *Dependencies) (Module, error) {
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
	interceptorLogger, err := initInterceptorLogger(d.KusciaConfig)
	if err != nil {
		nlog.Errorf("Init Kuscia API interceptor logger failed: %v", err)
		return nil, err
	}

	kusciaAPIConfig.InterceptorLog = interceptorLogger
	nlog.Debugf("Kuscia api config is %+v", kusciaAPIConfig)

	return &kusciaAPIModule{
		conf:         kusciaAPIConfig,
		kusciaClient: d.Clients.KusciaClient,
		kubeClient:   d.Clients.KubeClient,
	}, nil
}

func initInterceptorLogger(kusciaConf confloader.KusciaConfig) (*nlog.NLog, error) {
	logConfig := initLoggerConfig(kusciaConf, kusciaAPIInterceptorLoggerPath)
	logWriter, err := zlogwriter.New(logConfig)
	if err != nil {
		return nil, err
	}
	return nlog.NewNLog(nlog.SetWriter(logWriter), nlog.SetFormatter(nlog.NewGinLogFormatter())), nil
}

func (m kusciaAPIModule) Run(ctx context.Context) error {
	return commands.Run(ctx, m.conf, m.kusciaClient, m.kubeClient)
}

func (m kusciaAPIModule) WaitReady(ctx context.Context) error {
	timeoutTicker := time.NewTicker(30 * time.Second)
	defer timeoutTicker.Stop()
	checkTicker := time.NewTicker(100 * time.Millisecond)
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
			return fmt.Errorf("wait kuscia api ready timeout")
		}
	}
}

func (m kusciaAPIModule) Name() string {
	return "kusciaAPI"
}

func (m kusciaAPIModule) readyZ() bool {
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// init client tls config
	tlsConfig := m.conf.TLS
	if tlsConfig != nil {
		if m.conf.Protocol == common.TLS {
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
	tokenConfig := m.conf.Token
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
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, constants.LocalhostIP, m.conf.HTTPPort, constants.HealthAPI)
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

	grpcAddr := fmt.Sprintf("%s:%d", constants.LocalhostIP, m.conf.GRPCPort)
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

func RunKusciaAPIWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := NewShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "kusciaapi",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunKusciaAPI(runCtx, cancel, conf, shutdownEntry)
}

func RunKusciaAPI(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m, err := NewKusciaAPI(conf)
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
		nlog.Fatalf("KusciaApi wait ready failed: %v", err)
	}
	nlog.Info("KusciaApi is ready")
	return m
}
