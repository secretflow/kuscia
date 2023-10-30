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

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	flight2 "github.com/secretflow/kuscia/pkg/datamesh/flight"
	"github.com/secretflow/kuscia/pkg/datamesh/handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type grpcServerBean struct {
	config *config.DataMeshConfig
}

func NewGrpcServerBean(config *config.DataMeshConfig) *grpcServerBean { // nolint: golint
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
		grpc.ConnectionTimeout(time.Duration(s.config.ConnectTimeOut) * time.Second),
	}

	serverTLSConfig, err := tls.BuildServerTLSConfig(s.config.TLS.RootCA,
		s.config.TLS.ServerCert, s.config.TLS.ServerKey)
	if err != nil {
		nlog.Fatalf("Failed to init server tls config: %v", err)
	}
	creds := credentials.NewTLS(serverTLSConfig)
	opts = append(opts, grpc.Creds(creds))

	// listen on grpc port
	addr := fmt.Sprintf(":%d", s.config.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		nlog.Fatalf("Failed to listen on addr[%s]: %v", addr, err)
	}

	// register grpc server
	server := grpc.NewServer(opts...)
	// get operator bean
	domainDataService := service.NewDomainDataService(s.config)
	datasourceService := service.NewDomainDataSourceService(s.config)
	datamesh.RegisterDomainDataServiceServer(server, grpchandler.NewDomainDataHandler(domainDataService))
	datamesh.RegisterDomainDataSourceServiceServer(server, grpchandler.NewDomainDataSourceHandler(datasourceService))
	datamesh.RegisterDomainDataGrantServiceServer(server, grpchandler.NewDomainDataGrantHandler(service.NewDomainDataGrantService(s.config)))

	// register flight service
	if s.config.EnableDataProxy {
		dpConf := &flight2.DataProxyConfig{
			Addr:            s.config.DataProxyEndpoint,
			ClientTLSConfig: s.config.DataProxyTLSConfig,
		}
		metaSrv, err := flight2.NewMetaServer(domainDataService, datasourceService, dpConf)
		if err != nil {
			nlog.Fatalf("Failed to create meta server: %v", err)
		}
		flight.RegisterFlightServiceServer(server, grpchandler.NewFlightMetaHandler(metaSrv))
	}

	reflection.Register(server)

	nlog.Infof("Grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "DataMeshGrpcServer"
}
