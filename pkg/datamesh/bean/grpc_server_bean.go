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

	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/handler"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/v1handler/grpchandler"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
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
		grpc.ChainUnaryInterceptor(interceptor.UnaryRecoverInterceptor(pberrorcode.ErrorCode_DataMeshErrForUnexpected)),
		grpc.StreamInterceptor(interceptor.StreamRecoverInterceptor(pberrorcode.ErrorCode_DataMeshErrForUnexpected)),
		grpc.MaxRecvMsgSize(256 * 1024 * 1024), // 256MB
	}

	if !s.config.DisableTLS {
		serverTLSConfig, err := tls.BuildServerTLSConfig(s.config.TLS.RootCA,
			s.config.TLS.ServerCert, s.config.TLS.ServerKey)
		if err != nil {
			nlog.Fatalf("Failed to init server tls config: %v", err)
		}
		creds := credentials.NewTLS(serverTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}
	// set logger
	if s.config.InterceptorLog != nil {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptor.GrpcServerLoggingInterceptor(*s.config.InterceptorLog)))
	}
	// listen on grpc port
	addr := fmt.Sprintf("%s:%d", s.config.ListenAddr, s.config.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		nlog.Fatalf("Failed to listen on addr[%s]: %v", addr, err)
	}

	// register grpc server
	server := grpc.NewServer(opts...)
	// get operator bean
	domainDataService := service.NewDomainDataService(s.config)
	datasourceService := service.NewDomainDataSourceService(s.config, cmservice.Exporter.ConfigurationService())
	datamesh.RegisterDomainDataServiceServer(server, grpchandler.NewDomainDataHandler(domainDataService))
	datamesh.RegisterDomainDataSourceServiceServer(server, grpchandler.NewDomainDataSourceHandler(datasourceService))
	datamesh.RegisterDomainDataGrantServiceServer(server, grpchandler.NewDomainDataGrantHandler(service.NewDomainDataGrantService(s.config)))

	flight.RegisterFlightServiceServer(server, handler.NewDataMeshFlightHandler(domainDataService, datasourceService, s.config.DataProxyList))

	reflection.Register(server)

	nlog.Infof("Grpc server listening on %s", addr)

	// serve grpc
	return server.Serve(lis)
}

func (s *grpcServerBean) ServerName() string {
	return "DataMeshGrpcServer"
}
