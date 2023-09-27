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
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apiconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	apicode "github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domain"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domaindata"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domainroute"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/health"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/job"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/serving"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
)

type httpServerBean struct {
	frameworkconfig.FlagEnvConfigLoader
	config  *apiconfig.KusciaAPIConfig
	httpGin *gin.Engine
}

func NewHTTPServerBean(config *apiconfig.KusciaAPIConfig) *httpServerBean { // nolint: golint
	return &httpServerBean{
		FlagEnvConfigLoader: frameworkconfig.FlagEnvConfigLoader{
			EnableTLSFlag: true,
		},
		config: config,
	}
}

func (s *httpServerBean) Validate(errs *errorcode.Errs) {
}

func (s *httpServerBean) Init(e framework.ConfBeanRegistry) error {
	// tls config from config file
	tlsConfig := s.config.TLSConfig
	if tlsConfig != nil {
		// override tls flags by config
		s.TLSConfig.EnableTLS = true
		s.TLSConfig.CAPath = tlsConfig.RootCAFile
		s.TLSConfig.ServerCertPath = tlsConfig.ServerCertFile
		s.TLSConfig.ServerKeyPath = tlsConfig.ServerKeyFile
	}

	if s.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	eg, ok := e.(*engine.Engine)
	if !ok {
		return fmt.Errorf("unable convert e to engine.Engine")
	}

	httpGin := gin.New()
	httpGin.Use(gin.Recovery())

	tokenConfig := s.config.TokenConfig
	if tokenConfig != nil {
		token, err := utils.ReadToken(*tokenConfig)
		if err != nil {
			return err
		}
		httpGin.Use(interceptor.HTTPTokenAuthInterceptor(token))
	}
	s.httpGin = httpGin

	s.registerGroupRoutes(eg, httpGin)

	return nil
}

// Start httpServerBean
func (s *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	mux := http.NewServeMux()
	mux.Handle("/", s.httpGin)
	port := s.config.HTTPPort
	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.config.IdleTimeout) * time.Second,
	}

	// init server tls config
	if s.EnableTLS {
		serverTLSConfig, err := s.LoadServerTLSConfig()
		if err != nil {
			nlog.Errorf(err.Error())
			return err
		}
		server.TLSConfig = serverTLSConfig
		nlog.Infof("https server started on %s", addr)
		return server.ListenAndServeTLS(s.TLSConfig.ServerCertPath, s.TLSConfig.ServerKeyPath)
	}

	// server start on http server
	nlog.Infof("http server started on %s", addr)
	return server.ListenAndServe()
}

func (s *httpServerBean) ServerName() string {
	return "kusciaAPIHttpServer"
}

func (s *httpServerBean) registerGroupRoutes(eg *engine.Engine, httpEg *gin.Engine) {
	jobService := service.NewJobService(*s.config)
	domainService := service.NewDomainService(*s.config)
	routeService := service.NewDomainRouteService(*s.config)
	domainDataService := service.NewDomainDataService(*s.config)
	servingService := service.NewServingService(*s.config)
	healthSerice := service.NewHealthService()
	// define router groups
	groupsRouters := []*router.GroupRouters{
		// job group routes
		{
			Group: "api/v1/job",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, job.NewCreateJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, job.NewDeleteJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, job.NewQueryJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "stop",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, job.NewStopJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, job.NewBatchQueryJobStatusHandler(jobService))},
				},
			},
		},
		// domain group routes
		{
			Group: "api/v1/domain",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domain.NewCreateDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domain.NewDeleteDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domain.NewUpdateDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domain.NewQueryDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domain.NewBatchQueryDomainStatusHandler(domainService))},
				},
			},
		},
		// domain route routes
		{
			Group: "api/v1/route",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domainroute.NewCreateDomainRouteHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domainroute.NewDeleteDomainHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domainroute.NewQueryDomainRouteHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domainroute.NewBatchQueryDomainRouteStatusHandler(routeService))},
				},
			},
		},
		// domainData group routes
		{
			Group: "api/v1/domaindata",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewCreateDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewUpdateDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewDeleteDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewQueryDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewBatchQueryDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "list",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, domaindata.NewListDomainDataHandler(domainDataService))},
				},
			},
		},
		// serving group routes
		{
			Group: "api/v1/serving",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, serving.NewCreateServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, serving.NewUpdateServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, serving.NewDeleteServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, serving.NewQueryServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, serving.NewBatchQueryServingStatusHandler(servingService))},
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
					Handlers:     []gin.HandlerFunc{protoDecorator(eg, health.NewReadyHandler(healthSerice))},
				},
			},
		},
	}
	// register router groups to httpEg
	for _, gr := range groupsRouters {
		group := httpEg.Group(gr.Group, gr.GroupMiddleware...)
		for _, route := range gr.Routes {
			group.Handle(route.HTTPMethod, route.RelativePath, route.Handlers...)
		}
	}
}

// protoDecorator is used to wrap handler.
func protoDecorator(engine *engine.Engine, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.InterConnProtoDecoratorMaker(int32(apicode.ErrRequestValidate), int32(apicode.ErrForUnexpected))(engine, handler)
}
