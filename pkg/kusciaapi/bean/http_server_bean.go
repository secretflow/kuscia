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

//nolint:dupl
package bean

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/handler/httphandler/certificate"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	apiconfig "github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/appimage"
	handlerconfig "github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domain"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domaindata"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domaindatagrant"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domaindatasource"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/domainroute"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/health"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/job"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/log"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/middleware"
	"github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/serving"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/pkg/web/interceptor"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

type httpServerBean struct {
	config          *apiconfig.KusciaAPIConfig
	externalGinBean *beans.GinBean
	internalGinBean *beans.GinBean
	cmConfigService cmservice.IConfigService
}

func NewHTTPServerBean(config *apiconfig.KusciaAPIConfig, cmConfigService cmservice.IConfigService) *httpServerBean { // nolint: golint
	return &httpServerBean{
		config: config,
		externalGinBean: &beans.GinBean{
			ConfigLoader: &frameworkconfig.FlagEnvConfigLoader{
				Source: frameworkconfig.SourceEnv,
			},
			Port:          int(config.HTTPPort),
			Debug:         config.Debug,
			GinBeanConfig: convertToGinConf(config),
		},
		internalGinBean: &beans.GinBean{
			ConfigLoader: &frameworkconfig.FlagEnvConfigLoader{
				Source: frameworkconfig.SourceEnv,
			},
			IP:            "127.0.0.1",
			Port:          int(config.HTTPInternalPort),
			Debug:         config.Debug,
			GinBeanConfig: convertToInternalGinConf(config),
		},
		cmConfigService: cmConfigService,
	}
}

func (s *httpServerBean) Validate(errs *errorcode.Errs) {
	s.externalGinBean.Validate(errs)
	s.internalGinBean.Validate(errs)
}

func (s *httpServerBean) Init(e framework.ConfBeanRegistry) error {
	if s.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	if err := s.externalGinInit(e); err != nil {
		return err
	}
	if err := s.internalGinInit(e); err != nil {
		return err
	}
	middleware.InitConfig(s.config.ConfDir)
	return nil
}

func (s *httpServerBean) externalGinInit(e framework.ConfBeanRegistry) error {
	if err := s.externalGinBean.Init(e); err != nil {
		return err
	}
	// recover middleware
	s.externalGinBean.Use(gin.Recovery(), interceptor.HTTPServerLoggingInterceptor(*s.config.InterceptorLog))
	// auth token
	tokenConfig := s.config.Token
	if tokenConfig != nil {
		token, err := utils.ReadToken(*tokenConfig)
		if err != nil {
			return err
		}
		s.externalGinBean.Use(interceptor.HTTPTokenAuthInterceptor(token))
	}
	s.externalGinBean.Use(interceptor.HTTPSetMasterRoleInterceptor())
	s.registerGroupRoutes(e, s.externalGinBean)
	return nil
}

func (s *httpServerBean) internalGinInit(e framework.ConfBeanRegistry) error {
	if err := s.internalGinBean.Init(e); err != nil {
		return err
	}
	// recover middleware
	s.internalGinBean.Use(gin.Recovery())
	// auth Kuscia-Source header
	s.internalGinBean.Use(interceptor.HTTPSourceAuthInterceptor())
	// casbin permission
	s.internalGinBean.Use(middleware.PermissionMiddleWare)
	s.registerGroupRoutes(e, s.internalGinBean)
	return nil
}

// Start httpServerBean
func (s *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	errChan := make(chan error, 1)
	go func() {
		err := s.internalGinBean.Start(ctx, e)
		errChan <- err
	}()
	go func() {
		err := s.externalGinBean.Start(ctx, e)
		errChan <- err
	}()
	err := <-errChan
	nlog.Errorf("httpServerBean start failed, error:%s", err.Error())
	return err
}

func (s *httpServerBean) ServerName() string {
	return "kusciaAPIHttpServer"
}

func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry, bean *beans.GinBean) {
	jobService := service.NewJobService(s.config)
	domainService := service.NewDomainService(s.config)
	routeService := service.NewDomainRouteService(s.config)
	domainDataSourceService := service.NewDomainDataSourceService(s.config, s.cmConfigService)
	domainDataService := service.NewDomainDataService(s.config, s.cmConfigService)
	domainDataGrantService := service.NewDomainDataGrantService(s.config)
	servingService := service.NewServingService(s.config)
	appImageService := service.NewAppImageService(s.config)
	healthService := service.NewHealthService()
	certService := newCertService(s.config)
	configService := service.NewConfigService(s.config, s.cmConfigService)
	logService := service.NewLogService(s.config)
	// define router groups
	groupsRouters := []*router.GroupRouters{
		// job group routes
		{
			Group: "api/v1/job",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewCreateJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewDeleteJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewQueryJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "stop",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewStopJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewBatchQueryJobStatusHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "watch",
					Handlers:     []gin.HandlerFunc{job.NewWatchJobHandler(jobService).Handle},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "approve",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewApproveJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "suspend",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewSuspendJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "restart",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewRestartJobHandler(jobService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "cancel",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, job.NewCancelJobHandler(jobService))},
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
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domain.NewCreateDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domain.NewDeleteDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domain.NewUpdateDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domain.NewQueryDomainHandler(domainService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domain.NewBatchQueryDomainHandler(domainService))},
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
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domainroute.NewCreateDomainRouteHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domainroute.NewDeleteDomainHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domainroute.NewQueryDomainRouteHandler(routeService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domainroute.NewBatchQueryDomainRouteStatusHandler(routeService))},
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
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewCreateDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewUpdateDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewDeleteDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "deleteDataAndSource",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewDeleteDomainDataAndSourceHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewQueryDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewBatchQueryDomainDataHandler(domainDataService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "list",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindata.NewListDomainDataHandler(domainDataService))},
				},
			},
		},
		// domainDataSource routes
		{
			Group: "api/v1/domaindatasource",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewCreateDomainDataSourceHandler(domainDataSourceService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewUpdateDomainDataSourceHandler(domainDataSourceService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewDeleteDomainDataSourceHandler(domainDataSourceService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewQueryDomainDataSourceHandler(domainDataSourceService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewBatchQueryDomainDataSourceHandler(domainDataSourceService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "list",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatasource.NewListDomainDataSourceHandler(domainDataSourceService))},
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
					Handlers:     []gin.HandlerFunc{protoDecorator(e, serving.NewCreateServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, serving.NewUpdateServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, serving.NewDeleteServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, serving.NewQueryServingHandler(servingService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "status/batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, serving.NewBatchQueryServingStatusHandler(servingService))},
				},
			},
		},
		{
			Group: "api/v1/domaindatagrant",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewCreateDomainDataGrantHandler(domainDataGrantService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewUpdateDomainDataGrantHandler(domainDataGrantService))},
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
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewBatchQueryDomainDataGrantHandler(domainDataGrantService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "list",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, domaindatagrant.NewListDomainDataGrantHandler(domainDataGrantService))},
				},
			},
		},
		{
			Group: "api/v1/certificate",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "generate",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, certificate.NewGenerateKeyCertsHandler(certService))},
				},
			},
		},
		{
			Group: "api/v1/config",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handlerconfig.NewCreateConfigHandler(configService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handlerconfig.NewQueryConfigHandler(configService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handlerconfig.NewUpdateConfigHandler(configService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handlerconfig.NewDeleteConfigHandler(configService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handlerconfig.NewBatchQueryConfigHandler(configService))},
				},
			},
		},
		{
			Group: "api/v1/appimage",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, appimage.NewCreateAppImageHandler(appImageService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "update",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, appimage.NewUpdateAppImageHandler(appImageService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "delete",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, appimage.NewDeleteAppImageHandler(appImageService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, appimage.NewQueryAppImageHandler(appImageService))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "batchQuery",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, appimage.NewBatchQueryAppImageHandler(appImageService))},
				},
			},
		},
		{
			Group: "api/v1/log",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "task/query",
					Handlers:     []gin.HandlerFunc{log.NewQueryHandler(logService).Handle},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "node/query",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, log.NewQueryPodNodeHandler(logService))},
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

	// register router groups to httpEg
	for _, gr := range groupsRouters {
		group := bean.Group(gr.Group, gr.GroupMiddleware...)
		for _, route := range gr.Routes {
			group.Handle(route.HTTPMethod, route.RelativePath, route.Handlers...)
		}
	}
}

func newCertService(config *apiconfig.KusciaAPIConfig) cmservice.ICertificateService {
	var certValue = &atomic.Value{}
	var privateKey *rsa.PrivateKey
	switch config.RunMode {
	case common.RunModeMaster, common.RunModeAutonomy:
		if config.RootCA == nil || config.RootCAKey == nil {
			nlog.Fatalf("Init certificate service failed, error: config tls cert is nil.")
		}
		certValue.Store(config.RootCA)
		privateKey = config.RootCAKey
	case common.RunModeLite:
		certValue = config.DomainCertValue
		privateKey = config.DomainKey
	}
	certService := cmservice.NewCertificateService(&cmservice.CertificateServiceConfig{
		DomainCertValue: certValue,
		DomainKey:       privateKey,
	})
	return certService
}

// protoDecorator is used to wrap handler.
func protoDecorator(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.CustomProtoDecoratorMaker(setKusciaAPIErrorResp(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate), setKusciaAPIErrorResp(pberrorcode.ErrorCode_KusciaAPIErrForUnexpected))(e, handler)
}

func setKusciaAPIErrorResp(errCode pberrorcode.ErrorCode) func(flow *decorator.BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	return func(flow *decorator.BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {

		wrappedErr := fmt.Errorf("%s", errs)
		resp := v1alpha1.ErrorResponse{
			Status: &v1alpha1.Status{
				Code:    int32(errCode),
				Message: wrappedErr.Error(),
			},
		}
		bytes, _ := protojson.Marshal(&resp)
		response = &api.AnyStringProto{
			Content: string(bytes),
		}
		return response
	}
}

func convertToGinConf(conf *apiconfig.KusciaAPIConfig) beans.GinBeanConfig {
	var tlsConfig *beans.TLSServerConfig
	if conf.TLS != nil {
		if conf.Protocol == common.MTLS {
			tlsConfig = &beans.TLSServerConfig{
				CACert:     conf.TLS.RootCA,
				ServerCert: conf.TLS.ServerCert,
				ServerKey:  conf.TLS.ServerKey,
			}
		} else {
			tlsConfig = &beans.TLSServerConfig{
				ServerCert: conf.TLS.ServerCert,
				ServerKey:  conf.TLS.ServerKey,
			}
		}
	}
	return beans.GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &conf.ReadTimeout,
		WriteTimeout:    &conf.WriteTimeout,
		IdleTimeout:     &conf.IdleTimeout,
		MaxHeaderBytes:  nil,
		TLSServerConfig: tlsConfig,
	}
}

func convertToInternalGinConf(conf *apiconfig.KusciaAPIConfig) beans.GinBeanConfig {
	return beans.GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &conf.ReadTimeout,
		WriteTimeout:    &conf.WriteTimeout,
		IdleTimeout:     &conf.IdleTimeout,
		MaxHeaderBytes:  nil,
		TLSServerConfig: nil,
	}
}
