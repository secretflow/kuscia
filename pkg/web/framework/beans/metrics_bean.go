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
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/logs"
	"github.com/secretflow/kuscia/pkg/web/metrics"
)

type MetricsBean struct {
	framework.ConfigLoader
	// Configs
	Port   int  `name:"port" usage:"Metrics port - use default gin port when not set"`
	Enable bool `name:"enable" usage:"Metrics enable" default:"true"`

	ginServer *gin.Engine
}

func (b *MetricsBean) Validate(errs *errorcode.Errs) {
	if b.Port < 0 {
		errs.AppendErr(fmt.Errorf("metrics port: %v illegal", b.Port))
	}
}

func (b *MetricsBean) Init(e framework.ConfBeanRegistry) error {
	if !b.Enable {
		return nil
	}

	if bean, ok := e.GetBeanByName(framework.GinServerName); ok {
		if gBean, ok := bean.(*GinBean); ok {
			var engine *gin.Engine
			if b.Port == 0 {
				engine = gBean.Engine
			} else {
				// Create separated metrics server.
				engine = gin.New()
				engine.Use(gin.Recovery())
				b.ginServer = engine
				// Register MiddleWare and use the same output path as the gin.
				b.ginServer.Use(GinLogger(gBean.Logger, "metrics"))
			}
			if err := metrics.RegistGinWithRouter("gin", engine, "/metrics"); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("metrics can not get gin bean")
		}
	} else {
		return fmt.Errorf("metrics can not get gin bean")
	}

	return nil
}

func (b *MetricsBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	if !b.Enable {
		return nil
	}
	if b.Port != 0 {
		logs.GetLogger().Infof("metrics server started 0.0.0.0:%d", b.Port)
		return b.ginServer.Run(fmt.Sprintf(":%d", b.Port))
	}
	return nil
}
