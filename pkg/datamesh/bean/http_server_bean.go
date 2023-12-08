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

	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	dmconfig "github.com/secretflow/kuscia/pkg/datamesh/config"
	ecode "github.com/secretflow/kuscia/pkg/datamesh/errorcode"
	"github.com/secretflow/kuscia/pkg/datamesh/handler/httphandler/domaindata"
	"github.com/secretflow/kuscia/pkg/datamesh/handler/httphandler/domaindatagrant"
	"github.com/secretflow/kuscia/pkg/datamesh/handler/httphandler/domaindatasource"
	"github.com/secretflow/kuscia/pkg/datamesh/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/health"
	apisvc "github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
)

type httpServerBean struct {
	config  *dmconfig.DataMeshConfig
	ginBean beans.GinBean
}

func NewHTTPServerBean(config *dmconfig.DataMeshConfig) *httpServerBean { // nolint: golint
	return &httpServerBean{
		config: config,
		ginBean: beans.GinBean{
			ConfigLoader: &frameworkconfig.FlagEnvConfigLoader{
				Source: frameworkconfig.SourceEnv,
			},
			Port:          int(config.HTTPPort),
			Debug:         config.Debug,
			GinBeanConfig: convertToGinConf(config),
		},
	}
}

func (s *httpServerBean) Validate(errs *errorcode.Errs) {
	s.ginBean.Validate(errs)
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
	return "DataMeshHttpServer"
}

func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry) {
	domainDataService := service.NewDomainDataService(s.config)
	domainDataSourceService := service.NewDomainDataSourceService(s.config, cmservice.Exporter.ConfigurationService())
	domainDataGrantService := service.NewDomainDataGrantService(s.config)
	healthService := apisvc.NewHealthService()
	// define router groups
	groupsRouters := []*router.GroupRouters{
		// domainData group routes
		{
			Group: "api/v1/datamesh/domaindata",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewCreateDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewDeleteDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewQueryDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewUpdateDomainHandler(domainDataService))},
				},
			},
		},
		// domainData group routes
		{
			Group: "api/v1/datamesh/domaindatasource",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewQueryDomainDataSourceHandler(domainDataSourceService))},
				},
			},
		},
		{
			Group: "api/v1/datamesh/domaindatagrant",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewCreateDomainDataGrantHandler(domainDataGrantService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewDeleteDomainDataGrantHandler(domainDataGrantService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewQueryDomainDataGrantHandler(domainDataGrantService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewUpdateDomainSourceHandler(domainDataGrantService))},
				},
			},
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

func convertToGinConf(conf *dmconfig.DataMeshConfig) beans.GinBeanConfig {
	var tlsConf = &beans.TLSServerConfig{
		CACert:     conf.TLS.RootCA,
		ServerCert: conf.TLS.ServerCert,
		ServerKey:  conf.TLS.ServerKey,
	}
	return beans.GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &conf.ReadTimeout,
		WriteTimeout:    &conf.WriteTimeout,
		IdleTimeout:     &conf.IdleTimeout,
		MaxHeaderBytes:  nil,
		TLSServerConfig: tlsConf,
	}
}
