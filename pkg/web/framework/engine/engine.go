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
	"fmt"
	"os"
	"reflect"
	"runtime/debug"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/beans"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

// Engine is the framework's instance, it contains the configs, beans and app infos.
// Create an instance of Engine, by using New()/Default()
type Engine struct {
	info          *framework.AppConfig
	configs       *configContext
	beans         *beanContext
	routers       router.Routers
	groupsRouters router.GroupsRouters
}

// New returns a new blank Engine instance without any beans/config attached.
func New(conf *framework.AppConfig) *Engine {
	engine := &Engine{
		info: conf,
		configs: &configContext{
			Order:   make([]string, 0),
			Context: map[string]framework.Config{},
		},
		beans: &beanContext{
			Order:   make([]string, 0),
			Init:    map[string]bool{},
			Context: map[string]framework.Bean{},
		},
	}
	// default app info config(without set order)
	engine.configs.Context[framework.ConfName] = conf

	return engine
}

// Default returns an Engine instance with a gin bean,
// the gin server has logger and metrics already attached.
func Default(conf *framework.AppConfig) *Engine {
	engine := New(conf)

	// set default gin server
	g := &beans.GinBean{ConfigLoader: &config.FlagEnvConfigLoader{Source: config.SourceFlag}}
	engine.UseBeanWithConfig(framework.GinServerName, g)
	m := &beans.MetricsBean{ConfigLoader: &config.FlagEnvConfigLoader{Source: config.SourceFlag}}
	engine.UseBeanWithConfig(framework.MetricsName, m)
	return engine
}

// Run starts the Uitron framework.
// Note: if there is no bean in beans, Run will not block the calling goroutine.
func (e *Engine) RunCommand(ctx context.Context) error {
	// init pflag.CommandLine
	if pflag.CommandLine == nil {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	}
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	cmd := e.GetCommand(ctx)
	return cmd.Execute()
}

func (e *Engine) GetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     e.info.Name,
		Long:    e.info.Usage,
		Version: e.info.Version,
		Run: func(cmd *cobra.Command, args []string) {
			if err := e.runCmd(ctx, cmd, args); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	// Set flags of each config.
	fs := cmd.Flags()
	var namedFlagSets = &cliflag.NamedFlagSets{}

	// Register flags.
	e.configs.flags(namedFlagSets.FlagSet(e.info.Name))
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	cmd.MarkFlagFilename("config", "yaml", "yml", "json")
	return cmd
}

func (e *Engine) UseConfig(name string, conf framework.Config) error {
	if conf == nil {
		return fmt.Errorf("regist config %s error, invalid config", name)
	}
	return e.configs.register(name, conf)
}

func (e *Engine) UseConfigs(confs map[string]framework.Config) error {
	for name, configs := range confs {
		if err := e.configs.register(name, configs); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) UseBean(name string, conf framework.Config, bean framework.Bean) error {
	if bean == nil {
		return fmt.Errorf("regist bean %s error, invalid bean", name)
	}
	if conf != nil {
		// Register config corresponding to the bean.
		if err := e.UseConfig(name, conf); err != nil {
			return err
		}
	}
	return e.beans.register(name, bean)
}

func (e *Engine) UseBeanWithConfig(name string, confBean framework.Bean) error {
	c, ok := confBean.(framework.Config)
	if !ok {
		return fmt.Errorf("cannot use input confBean as type *Config in argument to UseBeanWithConfig")
	}
	return e.UseBean(name, c, confBean)
}

// UseRouters is a shortcut for setting default gin routers
func (e *Engine) UseRouters(routers router.Routers) {
	e.routers = append(e.routers, routers...)
}

// UseRouterGroups is a shortcut for setting default gin routers
func (e *Engine) UseRouterGroups(grprouters router.GroupsRouters) {
	e.groupsRouters = append(e.groupsRouters, grprouters...)
}

func (e *Engine) GetConfigByName(name string) (framework.Config, bool) {
	return e.configs.getByName(name)
}

func (e *Engine) GetBeanByName(name string) (framework.Bean, bool) {
	return e.beans.getByName(name)
}

func (e *Engine) Run(ctx context.Context) error {
	// 2. Config/flags processing and verification.
	errs := &errorcode.Errs{}
	e.configs.setValidate(errs)
	if len(*errs) > 0 {
		logs.GetLogger().Errorf("%v\n", utilerrors.NewAggregate(*errs))
		os.Exit(1)
	}

	// 3. Init Bean in sequence.
	errs = &errorcode.Errs{}
	e.beans.init(e, errs)
	if len(*errs) > 0 {
		logs.GetLogger().Errorf("%v\n", utilerrors.NewAggregate(*errs))
		os.Exit(1)
	}

	// 4. Get the default Gin bean and register the route for it.
	if gbean, ok := e.GetBeanByName(framework.GinServerName); !ok {
		logs.GetLogger().Info("default server bean not exist, skip router regist")
	} else {
		ginBean, ok := gbean.(*beans.GinBean)
		if !ok {
			logs.GetLogger().Error("unable convert gbean to GineBean")
			os.Exit(1)
		}
		for _, router := range e.routers {
			ginBean.RegisterRouter(router)
		}
		for _, groupRouters := range e.groupsRouters {
			ginBean.RegisterGroup(groupRouters)
		}
	}

	// 5. Start all beans and execute Start.
	engineCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	beanName, beanList := e.beans.listBeans()
	errChan := make(chan error, len(beanList))
	for i, b := range beanList {
		name := beanName[i]
		logs.GetLogger().Infof("Starting %s Bean: %s", reflect.TypeOf(b), name)
		go func(b framework.Bean) {
			// In case of panic() is called in user code.
			defer func() {
				if recError := recover(); recError != nil {
					err := fmt.Errorf("doBizProcess panic, err: %v", recError)
					logs.GetLogger().Errorf("Bean [%s] panic: %s", name, err)
					debug.PrintStack()
				}
			}()
			errChan <- b.Start(engineCtx, e)
		}(b)
	}

	// 6. Wait for the program to exit.
	for {
		select {
		case err := <-errChan:
			if err != nil {
				cancel()
				return err
			}
		case <-engineCtx.Done():
			return nil
		}
	}
}

func (e *Engine) runCmd(ctx context.Context, cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		logs.GetLogger().Info("arguments are not supported")
	}
	return e.Run(ctx)
}
