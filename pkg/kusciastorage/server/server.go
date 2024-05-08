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

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

// Server defines the server info which provide service.
type Server struct {
	config.FlagEnvConfigLoader

	Debug bool `name:"debug" usage:"server debug mode"`

	PublicPort   int `name:"public-port" usage:"public server port" default:"8080"`
	InternalPort int `name:"internal-port" usage:"internal server port" default:"8090"`

	ReadTimeout  int `name:"read-timeout" usage:"server read timeout in seconds" default:"300"`
	WriteTimeout int `name:"write-timeout" usage:"server write timeout in seconds" default:"300"`
	IdleTimeout  int `name:"idle-timeout" usage:"server idle timeout in seconds" default:"60"`

	LogLevel       string `name:"log-level" usage:"log level. supported list[debug,info,warn,error,fatal]" default:"info"`
	LogPath        string `name:"log-path" usage:"log path" default:"./logs/kuscia-storage.log"`
	LogMaxFileSize int    `name:"log-max-file-size" usage:"the max size of log file in MB, " default:"100"`
	LogMaxFiles    int    `name:"log-max-files" usage:"the max number of log files" default:"10"`

	ReqBodyMaxSize int `name:"request-body-max-size" usage:"the max size in bytes allowed for request body" default:"134217728"`

	PublicEngine   *gin.Engine
	InternalEngine *gin.Engine
}

// NewServer returns a server instance.
func NewServer() *Server {
	return &Server{}
}

// Validate is used to validate server config.
func (s *Server) Validate(errs *errorcode.Errs) {
	if s.PublicPort <= 0 {
		errs.AppendErr(fmt.Errorf("server public port %v is illegal", s.PublicPort))
	}

	if s.InternalPort <= 0 {
		errs.AppendErr(fmt.Errorf("server internal port %v is illegal", s.InternalPort))
	}

	if s.ReadTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server read timeout %v is illegal", s.ReadTimeout))
	}

	if s.WriteTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server write timeout %v is illegal", s.WriteTimeout))
	}

	if s.IdleTimeout < 0 {
		errs.AppendErr(fmt.Errorf("server idle timeout %v is illegal", s.IdleTimeout))
	}

	if s.ReqBodyMaxSize <= 0 {
		errs.AppendErr(fmt.Errorf("server request body max size %v is illegal", s.ReqBodyMaxSize))
	}
}

// RegisterGroup is used to register group for server.
func RegisterGroup(engine *gin.Engine, groupRouters *router.GroupRouters) {
	if groupRouters != nil {
		group := engine.Group(groupRouters.Group, groupRouters.GroupMiddleware...)
		for _, route := range groupRouters.Routes {
			group.Handle(route.HTTPMethod, route.RelativePath, route.Handlers...)
		}
	}
}

// Init is used to init server.
func (s *Server) Init(e framework.ConfBeanRegistry) error {
	w, err := zlogwriter.New(&nlog.LogConfig{
		LogLevel:      s.LogLevel,
		MaxFileSizeMB: s.LogMaxFileSize,
		LogPath:       s.LogPath,
		MaxFiles:      s.LogMaxFiles,
		Compress:      true,
	})
	if err != nil {
		return err
	}

	logs.Setup(nlog.SetWriter(w))
	nlog.Setup(nlog.SetWriter(w), nlog.SetFormatter(nlog.NewGinLogFormatter()))

	if s.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	eg, ok := e.(*engine.Engine)
	if !ok {
		return fmt.Errorf("unable convert e to engine.Engine")
	}

	pubGinEngine := gin.New()
	pubGinEngine.Use(gin.Recovery())
	pubGinEngine.Use(beans.GinLogger(nlog.DefaultLogger(), "public-server"))
	s.PublicEngine = pubGinEngine
	for _, groupRouters := range GetPublicRouterGroups(s, eg) {
		RegisterGroup(s.PublicEngine, groupRouters)
	}

	innerGinEngine := gin.New()
	innerGinEngine.Use(gin.Recovery())
	innerGinEngine.Use(beans.GinLogger(nlog.DefaultLogger(), "internal-server"))
	s.InternalEngine = innerGinEngine

	for _, groupRouters := range GetInternalRouterGroups(s, eg) {
		RegisterGroup(s.InternalEngine, groupRouters)
	}
	return nil
}

// Start is used to start server.
func (s *Server) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	errChan := make(chan error)
	// start public server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/", s.PublicEngine)
		sr := &http.Server{
			Addr:           fmt.Sprintf(":%d", s.PublicPort),
			Handler:        mux,
			ReadTimeout:    time.Duration(s.ReadTimeout) * time.Second,
			WriteTimeout:   time.Duration(s.WriteTimeout) * time.Second,
			IdleTimeout:    time.Duration(s.IdleTimeout) * time.Second,
			MaxHeaderBytes: 1 << 20,
		}

		nlog.Infof("server listen to public address 0.0.0.0:%d", s.PublicPort)
		errChan <- sr.ListenAndServe()
	}()

	// start internal server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/", s.InternalEngine)
		sr := &http.Server{
			Addr:           fmt.Sprintf(":%d", s.InternalPort),
			Handler:        mux,
			ReadTimeout:    time.Duration(s.ReadTimeout) * time.Second,
			WriteTimeout:   time.Duration(s.WriteTimeout) * time.Second,
			IdleTimeout:    time.Duration(s.IdleTimeout) * time.Second,
			MaxHeaderBytes: 1 << 20,
		}

		nlog.Infof("server listen to internal address 0.0.0.0:%d", s.InternalPort)
		errChan <- sr.ListenAndServe()
	}()
	return <-errChan
}
