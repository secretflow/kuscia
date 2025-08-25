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

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	frameworkconfig "github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

type HTTPServerBean struct {
	Config        *DiagnoseAPIConfig
	serverGinBean *beans.GinBean
	Service       *DiagnoseService
}

type DiagnoseAPIConfig struct {
	HTTPPort     int32 `yaml:"HTTPPort,omitempty"`
	Debug        bool
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

const (
	DIAGNOSE_SERVER_PORT = 8095
)

func NewHTTPServerBean(client *kubeconfig.KubeClients) *HTTPServerBean { // nolint: golint
	config := new(DiagnoseAPIConfig)
	config.HTTPPort = DIAGNOSE_SERVER_PORT
	return &HTTPServerBean{
		Config: config,
		serverGinBean: &beans.GinBean{
			ConfigLoader: &frameworkconfig.FlagEnvConfigLoader{
				Source: frameworkconfig.SourceEnv,
			},
			Port:          DIAGNOSE_SERVER_PORT,
			Debug:         config.Debug,
			GinBeanConfig: convertToGinConf(config),
		},
		Service: NewService(client),
	}
}

func (s *HTTPServerBean) Validate(errs *errorcode.Errs) {
	return
}

func convertToGinConf(conf *DiagnoseAPIConfig) beans.GinBeanConfig {
	return beans.GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &conf.ReadTimeout,
		WriteTimeout:    &conf.WriteTimeout,
		IdleTimeout:     &conf.IdleTimeout,
		MaxHeaderBytes:  nil,
		TLSServerConfig: nil,
	}
}

func (s *HTTPServerBean) Init(e framework.ConfBeanRegistry) error {
	if s.Config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := s.serverGinBean.Init(e); err != nil {
		return err
	}

	// recover middleware
	s.serverGinBean.Use(gin.Recovery())
	s.registerGroupRoutes(e, s.serverGinBean)
	return nil
}

func (s *HTTPServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	err := s.serverGinBean.Start(ctx, e)
	if err != nil {
		nlog.Errorf("HTTPServerBean start failed, error:%s", err.Error())
	}
	return err
}

func (s *HTTPServerBean) ServerName() string {
	return "kusciaDiagnoseHttpServer"
}

func (s *HTTPServerBean) registerGroupRoutes(e framework.ConfBeanRegistry, bean *beans.GinBean) {
	groupsRouters := []*router.GroupRouters{
		{
			Group: common.DiagnoseNetworkGroup,
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: common.DiagnoseMockPath,
					Handlers:     []gin.HandlerFunc{s.Service.Mock},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: common.DiagnoseSubmitReportPath,
					Handlers:     []gin.HandlerFunc{s.Service.SubmitReport},
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: common.DiagnoseHealthyPath,
					Handlers:     []gin.HandlerFunc{s.Service.Healthy},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: common.DiagnoseGetEnvoyLogByTask,
					Handlers:     []gin.HandlerFunc{s.Service.GetEnvoyLog},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: common.DiagnoseGetTaskInfo,
					Handlers:     []gin.HandlerFunc{s.Service.GetTask},
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

func (s *HTTPServerBean) Run(ctx context.Context) error {
	// new app engine
	appEngine := engine.New(&framework.AppConfig{
		Name:    "KusciaDiagnoseAPI",
		Usage:   "KusciaDiagnoseAPI",
		Version: meta.KusciaVersionString(),
	})
	serverName := s.ServerName()
	err := appEngine.UseBeanWithConfig(serverName, s)
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", serverName, err.Error())
	}

	// Run Server
	if err := appEngine.Run(ctx); err != nil {
		return fmt.Errorf("server run failed, %v", err.Error())
	}
	return nil
}

func (s *HTTPServerBean) WaitClose(ctx context.Context) {
	timeout := time.After(time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if s.Service.PeerDone {
				nlog.Infof("Received done signal, close server")
				return
			}
		case <-timeout:
			nlog.Infof("Reach server timeout, close server")
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *HTTPServerBean) WaitReport(ctx context.Context) []*diagnose.MetricItem {
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			if s.Service.RecReportDone {
				nlog.Infof("Receive report from peer")
				return s.Service.Report
			}
		case <-timeout:
			nlog.Infof("Wait peer report reach timeout, return nil")
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}
