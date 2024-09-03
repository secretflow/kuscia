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

	"github.com/gin-gonic/gin"

	cmconfig "github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/confmanager/handler/httphandler/certificate"
	"github.com/secretflow/kuscia/pkg/confmanager/interceptor"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/health"
	apisvc "github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	webinterceptor "github.com/secretflow/kuscia/pkg/web/interceptor"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

type httpServerBean struct {
	config        *cmconfig.ConfManagerConfig
	ginBean       beans.GinBean
	certService   service.ICertificateService
	configService service.IConfigService
}

func NewHTTPServerBean(config *cmconfig.ConfManagerConfig,
	certService service.ICertificateService,
	configService service.IConfigService) *httpServerBean { // nolint: golint
	return &httpServerBean{
		config: config,
		ginBean: beans.GinBean{
			Port:          int(config.HTTPPort),
			GinBeanConfig: convertToGinConf(config),
		},
		certService:   certService,
		configService: configService,
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
	s.ginBean.Use(gin.Recovery(), webinterceptor.HTTPServerLoggingInterceptor(*s.ginBean.Logger))
	if err := s.registerGroupRoutes(e); err != nil {
		return err
	}
	return nil
}

// Start httpServerBean
func (s *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	return s.ginBean.Start(ctx, e)
}

func (s *httpServerBean) ServerName() string {
	return "ConfManagerHTTPServer"
}

func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry) error {
	healthService := apisvc.NewHealthService()
	// define router groups
	groupsRouters := []*router.GroupRouters{
		{
			Group: "api/v1/configmanager/certificate",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "generate",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, certificate.NewGenerateKeyCertsHandler(s.certService))},
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
	return nil
}

// protoDecorator is used to wrap handler.
func protoDecorator(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.DefaultProtoDecoratorMaker(int32(pberrorcode.ErrorCode_ConfManagerErrRequestInvalidate), int32(pberrorcode.ErrorCode_ConfManagerErrForUnexpected))(e, handler)
}

func convertToGinConf(conf *cmconfig.ConfManagerConfig) beans.GinBeanConfig {
	return beans.GinBeanConfig{
		ReadTimeout:  &conf.ReadTimeout,
		WriteTimeout: &conf.WriteTimeout,
		IdleTimeout:  &conf.IdleTimeout,
		TLSServerConfig: &beans.TLSServerConfig{
			CACert:     conf.TLS.RootCA,
			ServerCert: conf.TLS.ServerCert,
			ServerKey:  conf.TLS.ServerKey,
		},
	}
}
