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
	"net/http"

	cmconfig "github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/handler/httphandler/certificate"
	"github.com/secretflow/kuscia/pkg/confmanager/handler/httphandler/configuration"
	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	ecode "github.com/secretflow/kuscia/pkg/datamesh/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/health"
	apisvc "github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	"github.com/secretflow/kuscia/pkg/web/framework/router"

	"github.com/gin-gonic/gin"
)

type httpServerBean struct {
	config  cmconfig.ConfManagerConfig
	ginBean beans.GinBean
}

func NewHTTPServerBean(config *cmconfig.ConfManagerConfig) *httpServerBean { // nolint: golint
	return &httpServerBean{
		config: *config,
		ginBean: beans.GinBean{
			Port:          int(config.HTTPPort),
			GinBeanConfig: convertToGinConf(config),
		},
	}
}

func (s *httpServerBean) Validate(errs *errorcode.Errs) {
	s.ginBean.Validate(errs)
	s.config.MustTLSEnables(errs)
}

func (s *httpServerBean) Init(e framework.ConfBeanRegistry) error {
	if err := s.ginBean.Init(e); err != nil {
		return err
	}
	s.registerGroupRoutes(e)
	return nil
}

// Start httpServerBean
func (s *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	return s.ginBean.Start(ctx, e)
}

func (s *httpServerBean) ServerName() string {
	return "ConfManagerHttpServer"
}

func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry) {
	certificateService, err := service.NewCertificateService(service.CertConfig{
		CertValue:  s.config.DomainCertValue,
		PrivateKey: s.config.DomainKey,
	})
	if err != nil {
		nlog.Fatalf("Failed to init certificate service : %v", err)
	}
	configurationService, err := service.NewConfigurationService(s.config)
	if err != nil {
		nlog.Fatalf("Failed to init configuration service : %v", err)
	}

	healthService := apisvc.NewHealthService()
	// define router groups
	groupsRouters := []*router.GroupRouters{
		// configuration group routes
		{
			Group: "api/v1/cm/configuration",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, configuration.NewCreateConfigurationHandler(configurationService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, configuration.NewQueryConfigurationHandler(configurationService))},
				},
			},
			GroupMiddleware: []gin.HandlerFunc{interceptor.HTTPTLSCertInfoInterceptor},
		},
		{
			Group: "api/v1/cm/certificate",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "generate",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, certificate.NewGenerateKeyCertsHandler(certificateService))},
				},
			},
			GroupMiddleware: []gin.HandlerFunc{interceptor.HTTPTLSCertInfoInterceptor},
		},
		// health group routes
		{
			Group: "",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: constants.HealthAPI,
					Handlers:     []gin.HandlerFunc{protoDecorator(e, health.NewReadyHandler(healthService))},
				},
			},
		},
	}
	// register group
	for _, gr := range groupsRouters {
		s.ginBean.RegisterGroup(gr)
	}
}

// protoDecorator is used to wrap handler.
func protoDecorator(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.InterConnProtoDecoratorMaker(int32(ecode.ErrRequestInvalidate), int32(ecode.ErrForUnexpected))(e, handler)
}

func convertToGinConf(conf *cmconfig.ConfManagerConfig) beans.GinBeanConfig {
	return beans.GinBeanConfig{
		Logger:         nil,
		ReadTimeout:    &conf.ReadTimeout,
		WriteTimeout:   &conf.WriteTimeout,
		IdleTimeout:    &conf.IdleTimeout,
		MaxHeaderBytes: nil,
		TLSServerConfig: &beans.TLSServerConfig{
			CACert:     conf.TLS.RootCA,
			ServerCert: conf.TLS.ServerCert,
			ServerKey:  conf.TLS.ServerKey,
		},
	}
}
