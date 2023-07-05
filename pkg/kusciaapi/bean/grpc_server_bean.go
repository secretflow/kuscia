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
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"

	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

type grpcServerBean struct {
	frameworkconfig.FlagEnvConfigLoader
	config config.KusciaAPIConfig
}

func NewGrpcServerBean(config *config.KusciaAPIConfig) *grpcServerBean { // nolint: golint
	return &grpcServerBean{
		FlagEnvConfigLoader: frameworkconfig.FlagEnvConfigLoader{
			EnableTLSFlag: true,
		},
		config: *config,
	}
}

func (s *grpcServerBean) Validate(errs *errorcode.Errs) {
}

func (s *grpcServerBean) Init(e framework.ConfBeanRegistry) error {
	// tls config from config file
	tlsConfig := s.config.TLSConfig
	if tlsConfig != nil {
		// override tls flags by config
		s.TLSConfig.EnableTLS = true
		s.TLSConfig.CAPath = tlsConfig.RootCAFile
		s.TLSConfig.ServerCertPath = tlsConfig.ServerCertFile
		s.TLSConfig.ServerKeyPath = tlsConfig.ServerKeyFile
	}
	return nil
}

// Start grpcServerBean
func (s *grpcServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	// init grpc server opts
	opts := []grpc.ServerOption{
		grpc.ConnectionTimeout(time.Duration(s.config.ConnectTimeOut) * time.Second),
	}
	if s.EnableTLS {
		// tls enabled
		serverTLSConfig, err := s.LoadServerTLSConfig()
		if err != nil {
			nlog.Fatalf("failed to init server tls config: %v", err)
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

	tokenConfig := s.config.TokenConfig
	if s.config.TokenConfig != nil {
		token, err := utils.ReadToken(*tokenConfig)
		if err != nil {
			return err
		}
		tokenInterceptor := grpc.UnaryInterceptor(interceptor.GrpcServerTokenInterceptor(token))
		opts = append(opts, tokenInterceptor)
		tokenStreamInterceptor := grpc.StreamInterceptor(interceptor.GrpcStreamServerTokenInterceptor(token))
		opts = append(opts, tokenStreamInterceptor)
	}

	// register grpc server
	server := grpc.NewServer(opts...)
	kusciaapi.RegisterJobServiceServer(server, grpchandler.NewJobHandler(service.NewJobService(s.config)))
	kusciaapi.RegisterDomainServiceServer(server, grpchandler.NewDomainHandler(service.NewDomainService(s.config)))
	kusciaapi.RegisterDomainRouteServiceServer(server, grpchandler.NewDomainRouteHandler(service.NewDomainRouteService(s.config)))
	kusciaapi.RegisterHealthServiceServer(server, grpchandler.NewHealthHandler(service.NewHealthService()))
	kusciaapi.RegisterDomainDataServiceServer(server, grpchandler.NewDomainDataHandler(service.NewDomainDataService(s.config)))
	nlog.Infof("grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "kusciaAPIGrpcServer"
}
