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

	"github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type grpcServerBean struct {
	config config.ConfManagerConfig
}

func NewGrpcServerBean(config *config.ConfManagerConfig) *grpcServerBean { // nolint: golint
	return &grpcServerBean{
		config: *config,
	}
}

func (s *grpcServerBean) Validate(errs *errorcode.Errs) {
	s.config.MustTLSEnables(errs)
}

func (s *grpcServerBean) Init(e framework.ConfBeanRegistry) error {
	return nil
}

// Start grpcServerBean
func (s *grpcServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	certificateService := service.Exporter.CertificateService()
	configurationService := service.Exporter.ConfigurationService()

	// init grpc server opts
	opts := []grpc.ServerOption{
		grpc.ConnectionTimeout(time.Duration(s.config.ConnectTimeout) * time.Second),
		grpc.UnaryInterceptor(interceptor.GRPCTLSCertInfoInterceptor),
	}
	// tls must enabled
	serverTLSConfig, err := tls.BuildServerTLSConfig(s.config.TLS.RootCA,
		s.config.TLS.ServerCert, s.config.TLS.ServerKey)
	if err != nil {
		nlog.Errorf("Failed to init server tls config: %v", err)
		return err
	}
	creds := credentials.NewTLS(serverTLSConfig)
	opts = append(opts, grpc.Creds(creds))

	// listen on grpc port
	addr := fmt.Sprintf(":%d", s.config.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		nlog.Errorf("Failed to listen on addr[%s]: %v", addr, err)
		return err
	}

	// register grpc server
	server := grpc.NewServer(opts...)
	confmanager.RegisterConfigurationServiceServer(server, grpchandler.NewConfigurationHandler(configurationService))
	confmanager.RegisterCertificateServiceServer(server, grpchandler.NewCertificateHandler(certificateService))
	reflection.Register(server)
	nlog.Infof("Grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "ConfManagerGrpcServer"
}
