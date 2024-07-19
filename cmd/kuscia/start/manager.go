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
	"fmt"
	"sync"
	"time"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/lock"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const moduleDefaultExitTimeout = time.Second * 3

type ModuleReadyHook func(ctx context.Context, modules map[string]modules.Module) error
type ModuleManager interface {
	// regist a new module
	Regist(name string, creator NewModuleFunc, modes ...common.RunModeType) error
	// set module dependency modules
	SetDependencies(name string, dependencies ...string) error

	// when [modules] are all ready, hook will called
	AddReadyHook(hook ModuleReadyHook, modules ...string) error

	// start to run all modules
	Start(ctx context.Context, mode common.RunModeType, conf *modules.ModuleRuntimeConfigs) error
}

type kusciaModuleReadyHook struct {
	// hook modules. when all modules are ready. hook function will called
	modules []string

	hook ModuleReadyHook

	isCalled bool

	// runtime context and cancel function
	ctx    context.Context
	cancel context.CancelFunc
}

type kusciaModuleManager struct {
	modules map[string]*moduleInfo

	readyHooks []*kusciaModuleReadyHook

	// runtime context and cancel function
	ctx    context.Context
	cancel context.CancelFunc

	// all modules had finished
	wg *sync.WaitGroup

	// newly created module use this chan to notify
	newlyModuleCh chan *moduleInfo

	// newly ready module use this chan to notify
	readyModuleCh chan *moduleInfo

	// run failed module use this chan to notify
	runFailedModuleCh chan *moduleInfo
}

func NewModuleManager() ModuleManager {
	return &kusciaModuleManager{
		modules:           map[string]*moduleInfo{},
		newlyModuleCh:     make(chan *moduleInfo, 100),
		readyModuleCh:     make(chan *moduleInfo, 100),
		runFailedModuleCh: make(chan *moduleInfo, 100),
		wg:                &sync.WaitGroup{},
	}
}

func (kmm *kusciaModuleManager) Regist(name string, creator NewModuleFunc, modes ...common.RunModeType) error {
	if creator == nil {
		return fmt.Errorf("module %s creator is nil", name)
	}

	if _, ok := kmm.modules[name]; ok {
		return fmt.Errorf("module(%s) had exists", name)
	}

	mset := map[common.RunModeType]bool{}
	for _, m := range modes {
		mset[m] = true
	}

	kmm.modules[name] = &moduleInfo{
		name:     name,
		creator:  creator,
		modes:    mset,
		instance: nil,
	}
	return nil
}

func (kmm *kusciaModuleManager) SetDependencies(name string, dependencies ...string) error {
	if module, ok := kmm.modules[name]; ok {
		for _, d := range dependencies {
			if _, ok = kmm.modules[d]; !ok {
				return fmt.Errorf("invalidate module %s", d)
			}
		}
		module.dependencies = append(module.dependencies, dependencies...)
		return nil
	}

	return fmt.Errorf("invalidate module %s", name)
}

func (kmm *kusciaModuleManager) AddReadyHook(hook ModuleReadyHook, modules ...string) error {
	if hook == nil {
		return errors.New("input hook function is nil")
	}
	for _, m := range modules {
		if _, ok := kmm.modules[m]; !ok {
			return fmt.Errorf("invalidate module name %s", m)
		}
	}

	kmm.readyHooks = append(kmm.readyHooks, &kusciaModuleReadyHook{
		modules:  modules,
		hook:     hook,
		isCalled: false,
	})

	return nil
}

func (kmm *kusciaModuleManager) Start(ctx context.Context, mode common.RunModeType, conf *modules.ModuleRuntimeConfigs) error {
	modules, moduleReverseDeps := kmm.initNeedStartModules(mode)

	// parent context do not use `ctx`; otherwise parent context(`ctx`) canceled, every module's context will cancel at once
	kmm.ctx, kmm.cancel = context.WithCancel(context.Background())
	defer kmm.cancel()

	// start no dependency modules
	for _, mc := range modules {
		if mc.isModuleDepReady(modules) {
			nlog.Infof("module(%s) no dep so start", mc.name)
			if err := kmm.runModule(kmm.ctx, mc, conf); err != nil {
				return err
			}
		}
	}

	for {
		var readyModule *moduleInfo
		select {
		case newCreateModule := <-kmm.newlyModuleCh:
			nlog.Infof("new created module")
			go func() {
				newCreateModule.readyError = newCreateModule.instance.WaitReady(ctx)
				kmm.readyModuleCh <- newCreateModule
			}()
		case readyModule = <-kmm.readyModuleCh:
			readyModule.isReadyWaitDone = true // only main coroutine can get/set isReadyWaitDone
			if readyModule.readyError != nil {
				nlog.Errorf("[Module] %s wait ready failed with %s, so start gracefulExit", readyModule.name, readyModule.readyError.Error())
				return kmm.gracefulExit(modules, moduleReverseDeps)
			}
			nlog.Infof("[Module] %s is ready now", readyModule.name)
		case <-kmm.runFailedModuleCh:
			return kmm.gracefulExit(modules, moduleReverseDeps)
		case <-ctx.Done():
			// exit atonce
			nlog.Infof("Got process exit signal, so start gracefulExit")
			return kmm.gracefulExit(modules, moduleReverseDeps)
		}

		if err := kmm.callbackHookFunctions(modules); err != nil {
			nlog.Infof("Callback hook function, so start gracefulExit")
			return kmm.gracefulExit(modules, moduleReverseDeps)
		}

		if readyModule == nil {
			continue
		}

		for _, name := range moduleReverseDeps[readyModule.name] {
			mc := modules[name]
			if mc.instance == nil { // module is not started
				if mc.isModuleDepReady(modules) {
					if err := kmm.runModule(kmm.ctx, mc, conf); err != nil {
						return err
					}
				}
			}
		}

		// if all module started break loop
		if kmm.isAllModulesStarted(modules) {
			break
		}
	}

	nlog.Info("[Module] all modules are startup")
	nlog.Info("++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	nlog.Info("Kuscia started success")
	nlog.Info("++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")

	select {
	case <-ctx.Done():
		nlog.Infof("Got process exit signal, so start gracefulExit")
		return kmm.gracefulExit(modules, moduleReverseDeps)
	case <-kmm.runFailedModuleCh:
		return kmm.gracefulExit(modules, moduleReverseDeps)
	case <-lock.NewWaitGroupChannel(kmm.wg):
		nlog.Infof("All module are finished")
	}

	return nil
}

func (kmm *kusciaModuleManager) callbackHookFunctions(ms map[string]*moduleInfo) error {
	for _, hf := range kmm.readyHooks {
		if hf.isCalled || hf.hook == nil {
			continue
		}
		isAllReady := true
		depModules := map[string]modules.Module{}
		for _, dep := range hf.modules {
			if m, ok := ms[dep]; ok {
				depModules[dep] = m.instance
				if !m.isReadyWaitDone {
					isAllReady = false
					break
				}
			}
		}

		if isAllReady {
			hf.isCalled = true

			hf.ctx, hf.cancel = context.WithCancel(kmm.ctx)
			if err := hf.hook(hf.ctx, depModules); err != nil {
				nlog.Warnf("Hook function callback failed with %s", err.Error())
				return err
			}
		}

	}

	return nil
}

func (kmm *kusciaModuleManager) cancelCallbacks(canExitModules map[string]bool) {
	for _, hf := range kmm.readyHooks {
		if !hf.isCalled {
			continue
		}

		isAllCanExit := false
		for _, dep := range hf.modules {
			if _, ok := canExitModules[dep]; ok {
				isAllCanExit = true
				break
			}
		}

		if isAllCanExit {
			hf.isCalled = false
			hf.cancel()
		}
	}
}

func (kmm *kusciaModuleManager) initNeedStartModules(mode common.RunModeType) (map[string]*moduleInfo, map[string][]string) {
	startModules := map[string]*moduleInfo{}
	for _, mc := range kmm.modules {
		if _, ok := mc.modes[mode]; ok {
			startModules[mc.name] = mc
		}
	}

	reverseDep := map[string][]string{}
	for _, mc := range startModules {
		reverseDep[mc.name] = []string{}
	}
	for _, mc := range startModules {
		for _, dep := range mc.dependencies {
			if _, ok := startModules[dep]; ok {
				reverseDep[dep] = append(reverseDep[dep], mc.name)
			}
		}
	}

	return startModules, reverseDep
}

func (kmm *kusciaModuleManager) isAllModulesStarted(modules map[string]*moduleInfo) bool {
	allModuleStarted := true
	for _, mc := range modules {
		if mc.instance == nil || !mc.isReadyWaitDone {
			allModuleStarted = false
		}
	}
	return allModuleStarted
}

func (kmm *kusciaModuleManager) runModule(ctx context.Context, mc *moduleInfo, conf *modules.ModuleRuntimeConfigs) error {
	nlog.Infof("Try to start module %s", mc.name)
	var err error
	mc.instance, err = mc.creator(conf)
	if err != nil {
		nlog.Errorf("[Module] %s init failed with error: %s", mc.name, err.Error())
		return err
	}
	nlog.Infof("[Module] %s is created", mc.name)

	mc.ctx, mc.cancel = context.WithCancel(ctx)

	kmm.newlyModuleCh <- mc

	kmm.wg.Add(1)
	mc.finishWG.Add(1)
	go func() {
		defer kmm.wg.Done()
		defer mc.finishWG.Done()
		err := mc.instance.Run(mc.ctx)

		if err != nil {
			nlog.Infof("[Module] %s is finished with err=%s", mc.name, err.Error())
			kmm.runFailedModuleCh <- mc
		} else {
			nlog.Infof("[Module] %s is successful finished", mc.name)
		}
	}()

	return nil
}

func (kmm *kusciaModuleManager) stepExit(canExitModules map[string]bool) error {
	wg := sync.WaitGroup{}
	for name, exited := range canExitModules {
		if exited {
			continue
		}
		mc := kmm.modules[name]

		nlog.Infof("[Module] %s notified to exit...", name)
		mc.cancel()
		canExitModules[name] = true
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			mc.finishWG.Wait()
		}(name)
	}
	// wait modules exited
	select {
	case <-time.After(moduleDefaultExitTimeout):
		return errors.New("some modules are not graceful exit, so exit process atonce")
	case <-lock.NewWaitGroupChannel(&wg):
		nlog.Infof("Current step modules are finished now")
	}

	return nil
}

func (kmm *kusciaModuleManager) gracefulExit(modules map[string]*moduleInfo, reverseDep map[string][]string) error {
	nlog.Infof("GracefulExit started...")
	// true: module exited, false module not exited
	canExitModules := map[string]bool{}

	// mark all modules (has no dependency) can exit
	for k, v := range reverseDep {
		if len(v) == 0 {
			canExitModules[k] = false
		}
	}

	// mark all not started module as exited
	for _, mc := range modules {
		if mc.instance == nil {
			canExitModules[mc.name] = true
		}
	}

	for {
		// cancel callbacks
		kmm.cancelCallbacks(canExitModules)

		// stop all modules with `false` value
		if err := kmm.stepExit(canExitModules); err != nil {
			return err
		}

		// current step modules are stopped, so some modules can exit next time
		hasNewModule := false
		for _, mc := range modules {
			if kmm.isModuleCanExit(mc, canExitModules, reverseDep[mc.name]) {
				canExitModules[mc.name] = false
				hasNewModule = true
			}
		}

		if !hasNewModule {
			break
		}
	}

	if len(canExitModules) != len(modules) {
		nlog.Warnf("No new module can exit, may be circular dependency")
	}

	return nil
}

// all dependencies are existed?
func (kmm *kusciaModuleManager) isModuleCanExit(mc *moduleInfo, canExitModules map[string]bool, reverseDeps []string) bool {
	if _, ok := canExitModules[mc.name]; ok {
		return false
	}

	canExit := true
	for _, dep := range reverseDeps {
		if isExited, ok := canExitModules[dep]; ok {
			if !isExited {
				canExit = false
				break
			}
		}
	}

	return canExit
}
