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

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type grpcServerBean struct {
	frameworkconfig.FlagEnvConfigLoader
	config config.DataMeshConfig
}

func NewGrpcServerBean(config *config.DataMeshConfig) *grpcServerBean { // nolint: golint
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
			nlog.Fatalf("Failed to init server tls config: %v", err)
		}
		creds := credentials.NewTLS(serverTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}

	// listen on grpc port
	addr := fmt.Sprintf(":%d", s.config.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		nlog.Fatalf("Failed to listen on addr[%s]: %v", addr, err)
	}

	// register grpc server
	server := grpc.NewServer(opts...)
	// get operator bean
	//
	datamesh.RegisterDomainDataServiceServer(server, grpchandler.NewDomainDataHandler(service.NewDomainDataService(s.config)))
	datamesh.RegisterDomainDataSourceServiceServer(server, grpchandler.NewDomainDataSourceHandler(service.NewDomainDataSourceService(s.config)))
	nlog.Infof("Grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "DataMeshGrpcServer"
}
