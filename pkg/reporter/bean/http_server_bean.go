// Copyright 2024 Ant Group Co., Ltd.
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

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"

	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/reporter/config"
	"github.com/secretflow/kuscia/pkg/reporter/handler"
	"github.com/secretflow/kuscia/pkg/reporter/service"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

type httpServerBean struct {
	config          *config.ReporterConfig
	ginBean         *beans.GinBean
	cmConfigService cmservice.IConfigService
}

func NewHTTPServerBean(config *config.ReporterConfig, cmConfigService cmservice.IConfigService) *httpServerBean { // nolint: golint
	return &httpServerBean{
		config: config,
		ginBean: &beans.GinBean{
			ConfigLoader: &frameworkconfig.FlagEnvConfigLoader{
				Source: frameworkconfig.SourceEnv,
			},
			IP:            "127.0.0.1",
			Port:          int(config.HTTPPort),
			Debug:         config.Debug,
			GinBeanConfig: convertToGinConf(config),
		},
		cmConfigService: cmConfigService,
	}
}

func (s *httpServerBean) Validate(errs *errorcode.Errs) {
	s.ginBean.Validate(errs)
}

func (s *httpServerBean) Init(e framework.ConfBeanRegistry) error {
	if s.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	if err := s.ginInit(e); err != nil {
		return err
	}
	return nil
}

func (s *httpServerBean) ginInit(e framework.ConfBeanRegistry) error {
	if err := s.ginBean.Init(e); err != nil {
		return err
	}
	s.registerGroupRoutes(e, s.ginBean)
	return nil
}

// Start httpServerBean
func (s *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	errChan := make(chan error, 1)
	go func() {
		err := s.ginBean.Start(ctx, e)
		errChan <- err
	}()
	err := <-errChan
	nlog.Errorf("httpServerBean start failed, error:%s", err.Error())
	return err
}

func (s *httpServerBean) ServerName() string {
	return "ReporterHttpServer"
}

func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry, bean *beans.GinBean) {
	reportService := service.NewReportService(s.config)
	healthService := service.NewHealthService(s.config)
	// define router groups
	groupsRouters := []*router.GroupRouters{
		// job group routes
		{
			Group: "report",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "progress",
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handler.NewReportProgressHandler(reportService))},
				},
			},
		},
		{
			Group: "",
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: constants.HealthAPI,
					Handlers:     []gin.HandlerFunc{protoDecorator(e, handler.NewHealthHandler(healthService))},
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

// protoDecorator is used to wrap handler.
func protoDecorator(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.CustomProtoDecoratorMaker(setReporterErrorResp(pberrorcode.ErrorCode_ReporterErrRequestInvalidate), setReporterErrorResp(pberrorcode.ErrorCode_ReporterErrForUnexptected))(e, handler)
}

func setReporterErrorResp(errCode pberrorcode.ErrorCode) func(flow *decorator.BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
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

func convertToGinConf(conf *config.ReporterConfig) beans.GinBeanConfig {

	return beans.GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &conf.ReadTimeout,
		WriteTimeout:    &conf.WriteTimeout,
		IdleTimeout:     &conf.IdleTimeout,
		MaxHeaderBytes:  nil,
		TLSServerConfig: nil,
	}

}
