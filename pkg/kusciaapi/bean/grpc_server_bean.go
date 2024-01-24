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
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type grpcServerBean struct {
	config *config.KusciaAPIConfig
}

func NewGrpcServerBean(config *config.KusciaAPIConfig) *grpcServerBean { // nolint: golint
	return &grpcServerBean{
		config: config,
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
	}
	if s.config.TLS != nil {
		serverTLSConfig, err := tls.BuildServerTLSConfig(s.config.TLS.RootCA, s.config.TLS.ServerCert, s.config.TLS.ServerKey)
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

	tokenConfig := s.config.Token
	if s.config.Token != nil {
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
	kusciaapi.RegisterDomainDataSourceServiceServer(server, grpchandler.NewDomainDataSourceHandler(service.NewDomainDataSourceService(s.config, cmservice.Exporter.ConfigurationService())))
	kusciaapi.RegisterServingServiceServer(server, grpchandler.NewServingHandler(service.NewServingService(s.config)))
	kusciaapi.RegisterDomainDataGrantServiceServer(server, grpchandler.NewDomainDataGrantHandler(service.NewDomainDataGrantService(s.config)))
	kusciaapi.RegisterCertificateServiceServer(server, grpchandler.NewCertificateHandler(newCertService(s.config)))
	reflection.Register(server)
	nlog.Infof("grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "kusciaAPIGrpcServer"
}
