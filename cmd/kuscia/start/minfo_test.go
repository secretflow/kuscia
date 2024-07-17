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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/cmd/kuscia/modules"
)

type mmIndex struct {
	nextCreateOrder int
	nextRunOrder    int
	nextStopOrder   int
	lock            sync.Mutex
}

type mockModule struct {
	createOrder int
	runOrder    int
	stopOrder   int
	name        string

	readyChan  chan struct{}
	readyError error
	runChan    chan struct{}
	runError   error
	idx        *mmIndex
}

func createMockModule(name string, idx *mmIndex) modules.Module {
	mm := &mockModule{
		name:        name,
		idx:         idx,
		createOrder: -1,
		runOrder:    -1,
		stopOrder:   -1,
		readyChan:   make(chan struct{}),
		runChan:     make(chan struct{}),
	}
	mm.idx.lock.Lock()
	defer mm.idx.lock.Unlock()
	mm.createOrder = mm.idx.nextCreateOrder
	mm.idx.nextCreateOrder++
	return mm
}

func (mm *mockModule) Run(ctx context.Context) error {
	if mm.idx != nil {
		func() {
			mm.idx.lock.Lock()
			defer mm.idx.lock.Unlock()
			mm.runOrder = mm.idx.nextRunOrder
			mm.idx.nextRunOrder++
		}()

		select {
		case <-mm.runChan:
		case <-ctx.Done():
		}

		if mm.runError != nil {
			return mm.runError
		}

		func() {
			mm.idx.lock.Lock()
			defer mm.idx.lock.Unlock()
			mm.stopOrder = mm.idx.nextStopOrder
			mm.idx.nextStopOrder++
		}()

	}
	return nil
}

func (mm *mockModule) WaitReady(ctx context.Context) error {
	if mm.readyChan != nil {
		select {
		case <-ctx.Done():
			return mm.readyError
		case <-mm.readyChan:
			return mm.readyError
		}
	}
	return nil
}

func (mm *mockModule) Name() string {
	return mm.name
}

func TestMInfo_IsModuleDepReady(t *testing.T) {
	m1 := &moduleInfo{}

	// no dep
	assert.True(t, m1.isModuleDepReady(map[string]*moduleInfo{}))

	// not create
	m2 := &moduleInfo{}
	m1.dependencies = append(m1.dependencies, "m2")
	assert.False(t, m1.isModuleDepReady(map[string]*moduleInfo{"m2": m2}))

	// not ready
	m2.instance = &mockModule{}
	assert.False(t, m1.isModuleDepReady(map[string]*moduleInfo{"m2": m2}))

	m2.isReadyWaitDone = true
	assert.True(t, m1.isModuleDepReady(map[string]*moduleInfo{"m2": m2}))
}
