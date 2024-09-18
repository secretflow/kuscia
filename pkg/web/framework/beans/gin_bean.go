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

package beans

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/pkg/web/logs"
	"github.com/secretflow/kuscia/pkg/web/metrics"
)

type GinBean struct {
	framework.ConfigLoader
	// Configs
	IP      string `name:"ip" usage:"Server bind ip" default:"0.0.0.0"`
	Port    int    `name:"port" usage:"Server port" default:"8080"`
	Debug   bool   `name:"debug" usage:"Debug mode"`
	LogPath string `name:"logpath" usage:"Gin Log path"`
	GinBeanConfig
	*gin.Engine
}

func (b *GinBean) Validate(errs *errorcode.Errs) {
	if b.Port <= 0 {
		errs.AppendErr(fmt.Errorf("server port: %v illegal", b.Port))
	}
}

func (b *GinBean) RegisterRouter(router *router.Router) {
	b.Engine.Handle(router.HTTPMethod, router.RelativePath, router.Handlers...)
}

func (b *GinBean) RegisterGroup(groupRouters *router.GroupRouters) {
	if groupRouters != nil {
		group := b.Engine.Group(groupRouters.Group, groupRouters.GroupMiddleware...)
		for _, route := range groupRouters.Routes {
			group.Handle(route.HTTPMethod, route.RelativePath, route.Handlers...)
		}
	}
}

func (b *GinBean) Init(e framework.ConfBeanRegistry) error {
	if b.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize gin (disable console automatic output).
	engine := gin.New()
	engine.Use(gin.Recovery())
	// Register probe.
	if conf, ok := e.GetConfigByName(framework.ConfName); ok {
		appconf, ok := conf.(*framework.AppConfig)
		if ok {
			prob := metrics.NewProbeInfo(appconf.Name, appconf.Version)
			prob.Use(engine)
		}
	}
	if b.LogPath != "" {
		logger, err := zlogwriter.New(
			&nlog.LogConfig{
				LogPath:       b.LogPath,
				LogLevel:      "INFO",
				MaxFileSizeMB: 512,
				MaxFiles:      10,
				Compress:      true,
			})
		if err != nil {
			return err
		}

		b.Logger = nlog.NewNLog(nlog.SetWriter(logger), nlog.SetFormatter(nlog.NewGinLogFormatter()))
	} else {
		b.Logger = nlog.DefaultLogger()
	}
	b.Engine = engine
	return nil
}

func (b *GinBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	mux := http.NewServeMux()
	mux.Handle("/", b.Engine)

	normalizeConfig(&b.GinBeanConfig)
	addr := fmt.Sprintf(":%d", b.Port)
	if b.IP != "" {
		addr = fmt.Sprintf("%s:%d", b.IP, b.Port)
	}
	s := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    time.Duration(*b.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(*b.WriteTimeout) * time.Second,
		MaxHeaderBytes: *b.MaxHeaderBytes,
		IdleTimeout:    time.Duration(*b.IdleTimeout) * time.Second,
	}

	// init server tls config
	if b.TLSServerConfig != nil {
		var err error
		s.TLSConfig, err = tls.BuildServerTLSConfig(b.TLSServerConfig.CACert, b.TLSServerConfig.ServerCert, b.TLSServerConfig.ServerKey)
		if err != nil {
			nlog.Errorf(err.Error())
			return err
		}
		nlog.Infof("https server started on %s", addr)
		return s.ListenAndServeTLS("", "")
	}

	logs.GetLogger().Infof("http server started %s", addr)
	return s.ListenAndServe()
}

type GinBeanConfig struct {
	Logger          *nlog.NLog
	ReadTimeout     *int
	WriteTimeout    *int
	IdleTimeout     *int
	MaxHeaderBytes  *int
	TLSServerConfig *TLSServerConfig
}

type TLSServerConfig struct {
	CACert     *x509.Certificate
	ServerCert *x509.Certificate
	ServerKey  *rsa.PrivateKey
}

var (
	defaultReadTimeout    = 10      // seconds
	defaultWriteTimeout   = 10      // seconds
	defaultIdleTimeout    = 300     // seconds
	defaultMaxHeaderBytes = 1 << 20 // 1MB
)

func defaultGinConfig() GinBeanConfig {
	return GinBeanConfig{
		Logger:          nil,
		ReadTimeout:     &defaultReadTimeout,
		WriteTimeout:    &defaultWriteTimeout,
		IdleTimeout:     &defaultIdleTimeout,
		MaxHeaderBytes:  &defaultMaxHeaderBytes,
		TLSServerConfig: nil,
	}
}

func normalizeConfig(conf *GinBeanConfig) {
	if conf.ReadTimeout == nil {
		conf.ReadTimeout = &defaultReadTimeout
	}
	if conf.WriteTimeout == nil {
		conf.WriteTimeout = &defaultWriteTimeout
	}
	if conf.IdleTimeout == nil {
		conf.IdleTimeout = &defaultIdleTimeout
	}
	if conf.MaxHeaderBytes == nil {
		conf.MaxHeaderBytes = &defaultMaxHeaderBytes
	}
}
