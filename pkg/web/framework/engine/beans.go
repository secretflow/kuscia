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

	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

type beanContext struct {
	sync.RWMutex
	// Initialization order.
	Order   []string
	Init    map[string]bool
	Context map[string]framework.Bean
}

// Register Bean.
func (b *beanContext) register(name string, bean framework.Bean) error {
	b.RWMutex.Lock()
	defer b.RWMutex.Unlock()
	if _, ok := b.Context[name]; ok {
		return fmt.Errorf("duplicate regist bean name:'%s'", name)
	}
	b.Context[name] = bean
	b.Init[name] = false
	b.Order = append(b.Order, name)
	return nil
}

func (b *beanContext) getByName(name string) (framework.Bean, bool) {
	b.RWMutex.RLock()
	defer b.RWMutex.RUnlock()
	if init, find := b.Init[name]; !find || !init {
		return nil, false
	}
	bean, ok := b.Context[name]
	return bean, ok
}

func (b *beanContext) init(e *Engine, errs *errorcode.Errs) {
	b.RWMutex.RLock()
	defer b.RWMutex.RUnlock()
	for _, name := range b.Order {
		if err := b.Context[name].Init(e); err != nil {
			errs.AppendErr(err)
		} else {
			b.Init[name] = true
		}
	}
}

func (b *beanContext) listBeans() ([]string, []framework.Bean) {
	b.RWMutex.RLock()
	defer b.RWMutex.RUnlock()
	result := make([]framework.Bean, 0, len(b.Order))
	names := make([]string, 0, len(b.Order))
	for name, bean := range b.Context {
		names = append(names, name)
		result = append(result, bean)
	}
	return names, result
}
