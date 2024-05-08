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

package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/pflag"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
)

func main() {
	defaultProtoDecorator := decorator.DefaultProtoDecoratorMaker(-1, -2)
	interConnProtoDecorator := decorator.InterConnProtoDecoratorMaker(-1, -2)
	// blank framework
	r := engine.New(&framework.AppConfig{
		Name:    "new",
		Usage:   "new framework without gin server and etc.",
		Version: "v0.0.1",
	})

	// use Default settings (initedClass)
	r = engine.Default(&framework.AppConfig{
		Name:    "demo",
		Usage:   "just a demo",
		Version: "v0.0.1",
	})
	// r.UseConfig("test", &config.FlagEnvConfig{
	// 	Config: &TestConfig{},
	// 	Source: "flag",
	// })

	logCfg := zlogwriter.InstallPFlags(pflag.CommandLine)
	logWriter, err := zlogwriter.New(logCfg)
	if err != nil {

	}
	nlog.Setup(nlog.SetWriter(logWriter))
	testB := NewDemoConfig()
	if err := r.UseConfig("demo", testB); err != nil {
		nlog.Fatalf("Init config err: %v", err)
	}

	testC := NewTestBean(testB)
	if err := r.UseBeanWithConfig("test", testC); err != nil {
		nlog.Fatalf("Init test bean err: %v", err)
	}

	// Metrics demo
	userCnt := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "demo_metrics",
			Name:      "user_count",
			Help:      "user login request count.",
		},
		[]string{"user"},
	)
	prometheus.Register(userCnt)
	userCnt.With(prometheus.Labels{"user": "aaa"}).Add(1)
	// err when default gin not init
	r.UseRouters(router.Routers{
		{
			HTTPMethod:   http.MethodPost,
			RelativePath: "/api/v1/test",
			Handlers:     []gin.HandlerFunc{defaultProtoDecorator(r, &HelloHandler{})},
		},
	})

	r.UseRouterGroups(router.GroupsRouters{
		{
			Group: "/api/v2",
			Routes: router.Routers{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "login",
					Handlers:     []gin.HandlerFunc{defaultProtoDecorator(r, GetNewLoginHander(userCnt))},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "test1",
					Handlers:     []gin.HandlerFunc{defaultProtoDecorator(r, &HelloHandler{})},
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: "test2",
					Handlers:     []gin.HandlerFunc{interConnProtoDecorator(r, &HelloHandler{})},
				},
			},
		},
	})

	if err := r.RunCommand(signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler())); err != nil {
		nlog.Fatalf("App run failed: %v", err)
	}
}
