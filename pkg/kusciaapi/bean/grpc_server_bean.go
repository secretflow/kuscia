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

package bean

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutil "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type grpcServerBean struct {
	config          *config.KusciaAPIConfig
	cmConfigService cmservice.IConfigService
}

func NewGrpcServerBean(config *config.KusciaAPIConfig, cmConfigService cmservice.IConfigService) *grpcServerBean { // nolint: golint
	return &grpcServerBean{
		config:          config,
		cmConfigService: cmConfigService,
	}
}

func (s *grpcServerBean) Validate(errs *errorcode.Errs) {
}

func (s *grpcServerBean) Init(e framework.ConfBeanRegistry) error {
	return nil
}

// Start grpcServerBean
func (s *grpcServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	// init grpc server opts
	opts := []grpc.ServerOption{
		grpc.ConnectionTimeout(time.Duration(s.config.ConnectTimeout) * time.Second),
		grpc.ChainUnaryInterceptor(interceptor.UnaryRecoverInterceptor(pberrorcode.ErrorCode_KusciaAPIErrForUnexpected)),
		grpc.StreamInterceptor(interceptor.StreamRecoverInterceptor(pberrorcode.ErrorCode_KusciaAPIErrForUnexpected)),
		grpc.MaxRecvMsgSize(256 * 1024 * 1024), // 256MB
	}
	if s.config.TLS != nil {
		serverTLSConfig, err := buildServerTLSConfig(s.config.TLS, s.config.Protocol)
		if err != nil {
			nlog.Fatalf("Failed to init server tls config: %v", err)
		}
		creds := credentials.NewTLS(serverTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}

	// listen on grpc port
	addr := fmt.Sprintf(":%d", s.config.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		nlog.Fatalf("failed to listen on addr[%s]: %v", addr, err)
	}
	opts = append(opts, grpc.ChainUnaryInterceptor(interceptor.GrpcServerLoggingInterceptor(*s.config.InterceptorLog)))
	// set token auth interceptor
	tokenConfig := s.config.Token
	if s.config.Token != nil {
		token, err := utils.ReadToken(*tokenConfig)
		if err != nil {
			return err
		}
		tokenInterceptor := grpc.ChainUnaryInterceptor(interceptor.GrpcServerTokenInterceptor(token))
		opts = append(opts, tokenInterceptor)
		tokenStreamInterceptor := grpc.ChainStreamInterceptor(interceptor.GrpcStreamServerTokenInterceptor(token))
		opts = append(opts, tokenStreamInterceptor)
	}
	// set master role interceptor
	opts = append(opts, grpc.ChainUnaryInterceptor(interceptor.GrpcServerMasterRoleInterceptor()))
	opts = append(opts, grpc.ChainStreamInterceptor(interceptor.GrpcStreamServerMasterRoleInterceptor()))

	// register grpc server
	server := grpc.NewServer(opts...)
	kusciaapi.RegisterJobServiceServer(server, grpchandler.NewJobHandler(service.NewJobService(s.config)))
	kusciaapi.RegisterDomainServiceServer(server, grpchandler.NewDomainHandler(service.NewDomainService(s.config)))
	kusciaapi.RegisterDomainRouteServiceServer(server, grpchandler.NewDomainRouteHandler(service.NewDomainRouteService(s.config)))
	kusciaapi.RegisterHealthServiceServer(server, grpchandler.NewHealthHandler(service.NewHealthService()))
	kusciaapi.RegisterDomainDataServiceServer(server, grpchandler.NewDomainDataHandler(service.NewDomainDataService(s.config, s.cmConfigService)))
	kusciaapi.RegisterDomainDataSourceServiceServer(server, grpchandler.NewDomainDataSourceHandler(service.NewDomainDataSourceService(s.config, s.cmConfigService)))
	kusciaapi.RegisterServingServiceServer(server, grpchandler.NewServingHandler(service.NewServingService(s.config)))
	kusciaapi.RegisterDomainDataGrantServiceServer(server, grpchandler.NewDomainDataGrantHandler(service.NewDomainDataGrantService(s.config)))
	kusciaapi.RegisterCertificateServiceServer(server, grpchandler.NewCertificateHandler(newCertService(s.config)))
	kusciaapi.RegisterConfigServiceServer(server, grpchandler.NewConfigHandler(service.NewConfigService(s.config, s.cmConfigService)))
	kusciaapi.RegisterAppImageServiceServer(server, grpchandler.NewAppImageHandler(service.NewAppImageService(s.config)))
	kusciaapi.RegisterLogServiceServer(server, grpchandler.NewLogHandler(service.NewLogService(s.config)))

	reflection.Register(server)
	nlog.Infof("grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "kusciaAPIGrpcServer"
}

func buildServerTLSConfig(config *frameworkconfig.TLSServerConfig, protocol common.Protocol) (*tls.Config, error) {

	if config == nil {
		return nil, fmt.Errorf("tls config is empty")
	}
	if protocol == common.MTLS {
		return tlsutil.BuildServerTLSConfig(config.RootCA, config.ServerCert, config.ServerKey)
	}
	return tlsutil.BuildServerTLSConfig(nil, config.ServerCert, config.ServerKey)
}
