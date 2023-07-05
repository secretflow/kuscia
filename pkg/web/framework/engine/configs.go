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
	"fmt"
	"sync"

	"github.com/spf13/pflag"

	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

// configContext provide a container for engine to save all beans
type configContext struct {
	sync.RWMutex
	Order   []string
	Context map[string]framework.Config
}

func (c *configContext) register(name string, conf framework.Config) error {
	if len(name) == 0 || conf == nil {
		return fmt.Errorf("invalidate name:'%s' config:%v", name, conf)
	}
	c.RWMutex.Lock()
	defer c.RWMutex.Unlock()
	if _, ok := c.Context[name]; ok {
		return fmt.Errorf("duplicate regist config name:'%s'", name)
	}
	c.Context[name] = conf
	c.Order = append(c.Order, name)
	return nil
}

// flags register all flags.
func (c *configContext) flags(fs *pflag.FlagSet) {
	c.RWMutex.Lock()
	defer c.RWMutex.Unlock()
	for name, conf := range c.Context {
		if loader, ok := conf.(framework.ConfigLoader); ok {
			loader.RegisterFlags(conf, name, fs)
		}
	}
}

// SetValidate calls Process/validate in order.
func (c *configContext) setValidate(errs *errorcode.Errs) {
	c.RWMutex.Lock()
	defer c.RWMutex.Unlock()
	for _, name := range c.Order {
		if loader, ok := c.Context[name].(framework.ConfigLoader); ok {
			loader.Process(c.Context[name], name, errs)
		}
		c.Context[name].Validate(errs)
	}
}

// get config.
func (c *configContext) getByName(name string) (conf framework.Config, ok bool) {
	c.RWMutex.RLock()
	defer c.RWMutex.RUnlock()
	conf, ok = c.Context[name]
	return
}
