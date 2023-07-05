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

	"github.com/secretflow/kuscia/pkg/interconn/bfia/handler"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
)

const (
	defaultListenPort       = 8084
	defaultReadWriteTimeout = 30
	defaultIdleTimeout      = 30
)

// httpServerBean defines the http server bean.
type httpServerBean struct {
	*handler.ResourcesManager
	Debug        bool
	ListenPort   int
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int

	HTTPEngine *gin.Engine
}

// NewHTTPServerBean returns a http server bean.
func NewHTTPServerBean(rm *handler.ResourcesManager) *httpServerBean { // nolint: golint
	return &httpServerBean{
		ResourcesManager: rm,
		ListenPort:       defaultListenPort,
		ReadTimeout:      defaultReadWriteTimeout,
		WriteTimeout:     defaultReadWriteTimeout,
		IdleTimeout:      defaultIdleTimeout,
	}
}

// Validate is used to validate server config.
func (b *httpServerBean) Validate(errs *errorcode.Errs) {
	if b.ListenPort <= 0 {
		errs.AppendErr(fmt.Errorf("server listen port %v is illegal", b.ListenPort))
	}

	if b.ReadTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server read timeout %v is illegal", b.ReadTimeout))
	}

	if b.WriteTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server write timeout %v is illegal", b.WriteTimeout))
	}

	if b.IdleTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server idle timeout %v is illegal", b.IdleTimeout))
	}
}

// Init is used to init server.
func (b *httpServerBean) Init(e framework.ConfBeanRegistry) error {
	if b.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	eg, ok := e.(*engine.Engine)
	if !ok {
		return fmt.Errorf("unable convert e to engine.Engine")
	}

	httpEngine := gin.New()
	httpEngine.Use(gin.Recovery())
	b.HTTPEngine = httpEngine
	for _, groupRouters := range b.buildGroupRouters(eg) {
		b.registerGroupRoutes(groupRouters)
	}

	return nil
}

// registerGroupRoutes is used to register group routes for server.
func (b *httpServerBean) registerGroupRoutes(groupRouters *router.GroupRouters) {
	if groupRouters != nil {
		group := b.HTTPEngine.Group(groupRouters.Group, groupRouters.GroupMiddleware...)
		for _, route := range groupRouters.Routes {
			group.Handle(route.HTTPMethod, route.RelativePath, route.Handlers...)
		}
	}
}

// Start is used to start server.
func (b *httpServerBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	mux := http.NewServeMux()
	mux.Handle("/", b.HTTPEngine)
	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", b.ListenPort),
		Handler:        mux,
		ReadTimeout:    time.Duration(b.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(b.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(b.IdleTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	nlog.Infof("Start bfia http server, listening to address 0.0.0.0:%d", b.ListenPort)
	return s.ListenAndServe()
}

// buildGroupRouters builds group routers.
func (b *httpServerBean) buildGroupRouters(engine *engine.Engine) router.GroupsRouters {
	return []*router.GroupRouters{
		{
			Group:           "/v1/interconn/schedule/job",
			GroupMiddleware: nil,
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "create",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewCreateJobHandler(b.ResourcesManager))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "stop",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewStopJobHandler(b.ResourcesManager))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "start",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewStartJobHandler(b.ResourcesManager))},
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: "status_all",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewQueryJobStatusAllHandler(b.ResourcesManager))},
				},
			},
		},
		{
			Group:           "/v1/interconn/schedule/task",
			GroupMiddleware: nil,
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "start",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewStartTaskHandler(b.ResourcesManager))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "stop",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewStopTaskHandler(b.ResourcesManager))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "poll",
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, handler.NewPollTaskStatusHandler(b.ResourcesManager))},
				},
			},
		},
	}
}

// protoDecorator is used to decorate handler.
func protoDecorator(engine *engine.Engine, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.InterConnProtoDecoratorMaker(http.StatusBadRequest, http.StatusBadRequest)(engine, handler)
}
