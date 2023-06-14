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

package engine

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestEngine(t *testing.T) {
	// use Default settings (initedClass)
	r := Default(&framework.AppConfig{
		Name:    "demo",
		Usage:   "just a demo",
		Version: "v0.0.1",
	})

	testB := NewTestBean()
	assert.NoError(t, r.UseConfig("demo", testB))
	assert.NoError(t, r.UseConfigs(map[string]framework.Config{"demo1": testB}))

	assert.NoError(t, r.UseBeanWithConfig("test", testB))
	assert.NoError(t, r.UseBean("testb", nil, testB))

	_, ok := r.GetConfigByName("demo")
	assert.True(t, ok)
	_, ok = r.GetBeanByName("test")
	assert.True(t, !ok)

	r.UseRouters(router.Routers{
		{
			HTTPMethod:   http.MethodPost,
			RelativePath: "/api/v1/test",
		},
	})

	r.UseRouterGroups(router.GroupsRouters{
		{
			Group: "/api/v2",
			Routes: router.Routers{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: "test1",
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: "test2",
				},
			},
		},
	})

	r.SetArgs([]string{"test"})
	r.SetPreRunFunc(func(cmd *cobra.Command, args []string) error { return nil })

	stopCh := make(chan struct{}, 1)
	ctx := signals.NewKusciaContextWithStopCh(stopCh)
	go func() {
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()

	assert.NoError(t, r.Run(ctx))
	_, ok = r.GetBeanByName("test")
	assert.True(t, ok)
}

type TestBean struct {
	framework.ConfigLoader
}

func NewTestBean() *TestBean {
	return &TestBean{
		ConfigLoader: &config.FlagEnvConfigLoader{Source: config.SourceFlag},
	}
}

func (b *TestBean) Validate(errs *errorcode.Errs) {

}

func (b *TestBean) Init(e framework.ConfBeanRegistry) error {

	return nil
}

func (b *TestBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {

	return nil
}
