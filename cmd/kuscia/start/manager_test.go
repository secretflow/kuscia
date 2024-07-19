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

package start

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/common"
)

func TestModuleManager_Regist(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	assert.Error(t, m.Regist("m1", nil, common.RunModeAutonomy))

	creator := func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
		return &mockModule{}, nil
	}

	assert.NoError(t, m.Regist("m1", creator, common.RunModeAutonomy))
	assert.Error(t, m.Regist("m1", creator, common.RunModeAutonomy))
}

func TestModuleManager_SetDependencies(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	creator := func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
		return &mockModule{}, nil
	}

	assert.NoError(t, m.Regist("m1", creator, common.RunModeAutonomy))

	assert.Error(t, m.SetDependencies("m2", "m1"))
	assert.Error(t, m.SetDependencies("m1", "m2"))

	assert.NoError(t, m.Regist("m2", creator, common.RunModeAutonomy))
	assert.NoError(t, m.SetDependencies("m1", "m2"))

	kmm, ok := m.(*kusciaModuleManager)
	assert.True(t, ok)
	assert.NotNil(t, kmm)

	assert.Len(t, kmm.modules, 2)
	assert.NotNil(t, kmm.modules["m1"])
	assert.NotNil(t, kmm.modules["m2"])
	assert.Len(t, kmm.modules["m1"].dependencies, 1)
	assert.Equal(t, "m2", kmm.modules["m1"].dependencies[0])
}

func TestModuleManager_AddReadyHook(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	assert.Error(t, m.AddReadyHook(nil, "no-exists"))

	hook := func(ctx context.Context, modules map[string]modules.Module) error {
		return nil
	}
	assert.Error(t, m.AddReadyHook(hook, "no-exists"))

	creator := func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
		return &mockModule{}, nil
	}

	assert.NoError(t, m.Regist("m1", creator, common.RunModeAutonomy))
	assert.NoError(t, m.AddReadyHook(hook, "m1"))
}

func getModule(t *testing.T, m ModuleManager, name string) *mockModule {
	kmm, _ := m.(*kusciaModuleManager)
	assert.NotNil(t, kmm)
	assert.NotNil(t, kmm.modules[name])
	if kmm.modules[name].instance != nil {
		if mm, ok := kmm.modules[name].instance.(*mockModule); ok {
			return mm
		}
	}

	return nil
}

func TestModuleManager_Start_normal(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	idx := &mmIndex{}

	creator := func(name string) NewModuleFunc {
		return func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
			return createMockModule(name, idx), nil
		}
	}

	assert.NoError(t, m.Regist("m1", creator("m1"), common.RunModeAutonomy))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	normalModule := func(name string) {
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second, func() (done bool, err error) {
			return getModule(t, m, name) != nil, nil
		}))
		close(getModule(t, m, name).readyChan)
		time.Sleep(100 * time.Millisecond)
		close(getModule(t, m, name).runChan)
	}

	go normalModule("m1")

	assert.NoError(t, m.Start(ctx, common.RunModeAutonomy, nil))
	assert.Equal(t, 1, idx.nextCreateOrder)
	assert.Equal(t, 1, idx.nextRunOrder)
	assert.Equal(t, 1, idx.nextStopOrder)
}

func TestModuleManager_Start_withdep(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	idx := &mmIndex{}

	creator := func(name string) NewModuleFunc {
		return func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
			return createMockModule(name, idx), nil
		}
	}

	assert.NoError(t, m.Regist("m1", creator("m1"), common.RunModeAutonomy))
	assert.NoError(t, m.Regist("m2", creator("m2"), common.RunModeAutonomy))

	// start order: m2 --> m1
	assert.NoError(t, m.SetDependencies("m1", "m2"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	normalModule := func(name string) {
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second, func() (bool, error) {
			return getModule(t, m, name) != nil, nil
		}))
		time.Sleep(100 * time.Millisecond) // wait module is running
		close(getModule(t, m, name).readyChan)
	}

	go normalModule("m1")
	go normalModule("m2")
	go func() {
		kmm, _ := m.(*kusciaModuleManager)
		assert.NotNil(t, kmm)
		assert.NotNil(t, kmm.modules["m1"])
		assert.NotNil(t, kmm.modules["m2"])
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second*2, func() (bool, error) {
			return kmm.modules["m1"].isReadyWaitDone && kmm.modules["m2"].isReadyWaitDone, nil
		}))

		cancel()
	}()

	assert.NoError(t, m.Start(ctx, common.RunModeAutonomy, nil))
	assert.Equal(t, 2, idx.nextCreateOrder)
	assert.Equal(t, 2, idx.nextRunOrder)
	assert.Equal(t, 2, idx.nextStopOrder)

	m1 := getModule(t, m, "m1")
	assert.NotNil(t, m1)
	assert.Equal(t, 1, m1.createOrder)
	assert.Equal(t, 1, m1.runOrder)
	assert.Equal(t, 0, m1.stopOrder)

	m2 := getModule(t, m, "m2")
	assert.NotNil(t, m2)
	assert.Equal(t, 0, m2.createOrder)
	assert.Equal(t, 0, m2.runOrder)
	assert.Equal(t, 1, m2.stopOrder)
}

func TestModuleManager_Start_readyFailed(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	idx := &mmIndex{}

	creator := func(name string) NewModuleFunc {
		return func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
			return createMockModule(name, idx), nil
		}
	}

	assert.NoError(t, m.Regist("m1", creator("m1"), common.RunModeAutonomy))
	assert.NoError(t, m.Regist("m2", creator("m2"), common.RunModeAutonomy))

	// start order: m2 --> m1
	assert.NoError(t, m.SetDependencies("m1", "m2"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errorModule := func(name string) {
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second, func() (bool, error) {
			return getModule(t, m, name) != nil, nil
		}))
		getModule(t, m, name).readyError = errors.New("start failed")
		close(getModule(t, m, name).readyChan)
	}

	go errorModule("m2")

	assert.NoError(t, m.Start(ctx, common.RunModeAutonomy, nil))
	assert.Equal(t, 1, idx.nextCreateOrder)
	assert.Equal(t, 1, idx.nextRunOrder)
	assert.Equal(t, 1, idx.nextStopOrder)

	m1 := getModule(t, m, "m1")
	assert.Nil(t, m1)

	m2 := getModule(t, m, "m2")
	assert.NotNil(t, m2)
	assert.Equal(t, 0, m2.createOrder)
	assert.Equal(t, 0, m2.runOrder)
	assert.Equal(t, 0, m2.stopOrder)
}

func TestModuleManager_Start_runFailed(t *testing.T) {
	t.Parallel()
	m := NewModuleManager()
	assert.NotNil(t, m)

	idx := &mmIndex{}

	creator := func(name string) NewModuleFunc {
		return func(*modules.ModuleRuntimeConfigs) (modules.Module, error) {
			return createMockModule(name, idx), nil
		}
	}

	assert.NoError(t, m.Regist("m1", creator("m1"), common.RunModeAutonomy))
	assert.NoError(t, m.Regist("m2", creator("m2"), common.RunModeAutonomy))

	// start order: m2 --> m1
	assert.NoError(t, m.SetDependencies("m1", "m2"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	normalModule := func(name string) {
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second, func() (bool, error) {
			return getModule(t, m, name) != nil, nil
		}))
		time.Sleep(100 * time.Millisecond)
		close(getModule(t, m, name).readyChan)
	}

	runFailedModule := func(name string) {
		assert.NoError(t, wait.PollImmediate(time.Millisecond*50, time.Second, func() (bool, error) {
			return getModule(t, m, name) != nil, nil
		}))
		close(getModule(t, m, name).readyChan)
		time.Sleep(100 * time.Millisecond)
		getModule(t, m, name).runError = errors.New("run failed")
		close(getModule(t, m, name).runChan)
	}

	go runFailedModule("m1")
	go normalModule("m2")

	assert.NoError(t, m.Start(ctx, common.RunModeAutonomy, nil))
	assert.Equal(t, 2, idx.nextCreateOrder)
	assert.Equal(t, 2, idx.nextRunOrder)
	assert.Equal(t, 1, idx.nextStopOrder)

	m1 := getModule(t, m, "m1")
	assert.NotNil(t, m1)
	assert.Equal(t, 1, m1.createOrder)
	assert.Equal(t, 1, m1.runOrder)
	assert.Equal(t, -1, m1.stopOrder)

	m2 := getModule(t, m, "m2")
	assert.NotNil(t, m2)
	assert.Equal(t, 0, m2.createOrder)
	assert.Equal(t, 0, m2.runOrder)
	assert.Equal(t, 0, m2.stopOrder)
}
