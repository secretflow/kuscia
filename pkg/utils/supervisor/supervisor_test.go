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

package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/stretchr/testify/assert"
)

type FackCmd struct {
	startMock func() error
	waitMock  func() error
}

func (fc *FackCmd) Start() error {
	nlog.Info("fack cmd.start")
	if fc.startMock != nil {
		return fc.startMock()
	}
	return nil
}

func (fc *FackCmd) Wait() error {
	nlog.Info("fack cmd.wait")
	if fc.waitMock != nil {
		return fc.waitMock()
	}
	return nil
}

func newMockCmd(startMock func() error, waitMock func() error) Cmd {

	fack := FackCmd{
		startMock: startMock,
		waitMock:  waitMock,
	}
	return &fack
}

func TestNewSupervisor(t *testing.T) {
	var sp *Supervisor

	// normal setup
	sp = NewSupervisor("xyz", []int{1, 3}, 10)
	assert.Equal(t, sp.maxRestartCount, 10)
	assert.Equal(t, sp.restartIntervalIndex, 0)
	assert.Equal(t, sp.tag, "xyz")
	assert.NotEmpty(t, sp.minRunningTimeMS)
	assert.Equal(t, len(sp.restartIntervalMS), 2)
	assert.Equal(t, sp.restartIntervalMS[0], 1)
	assert.Equal(t, sp.restartIntervalMS[1], 3)

	// default setup
	sp = NewSupervisor("xyz", nil, -1)
	assert.True(t, sp.maxRestartCount > 100000000)
	assert.Equal(t, sp.restartIntervalIndex, 0)
	assert.Equal(t, sp.tag, "xyz")
	assert.Equal(t, len(sp.restartIntervalMS), len(defaultRestartIntervalMS))
}

func TestSupervisorRun_Startup(t *testing.T) {

	// startup failed
	sp := NewSupervisor("Startup", []int{0, 100}, 3)
	ctx := context.Background()

	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return nil
	})

	assert.NotNil(t, err)

}

func TestSupervisorRun_StartFailed(t *testing.T) {
	sp := NewSupervisor("StartFailed", []int{0, 100}, 3)
	ctx := context.Background()

	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(func() error {
			return errors.New("start failed")
		}, nil)
	})

	assert.NotNil(t, err)
}

func TestSupervisorRun_FirstStartFailed(t *testing.T) {

	sp := NewSupervisor("FirstStartFailed", []int{0, 100}, 3)
	ctx := context.Background()

	count := 0
	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(nil, func() error {
			count++
			time.Sleep(100 * time.Millisecond)
			return nil
		})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 1, count)
}

func TestSupervisorRun_MaxFailedCount(t *testing.T) {
	sp := NewSupervisor("MaxFailedCount", []int{0, 100}, 3)
	sp.minRunningTimeMS = 1000
	ctx := context.Background()

	count := 0
	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(nil, func() error {
			count++
			if count == 1 {
				time.Sleep(1100 * time.Millisecond)
			} else {
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 5, count)
}

func TestSupervisorRun_RestartAlways(t *testing.T) {
	sp := NewSupervisor("RestartAlways", []int{0, 100}, 3)
	sp.minRunningTimeMS = 1000
	ctx := context.Background()

	count := 0
	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(nil, func() error {
			count++
			if count <= 2 {
				time.Sleep(1100 * time.Millisecond)
			} else {
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 6, count)
}

func TestSupervisorRun_WaitFailed(t *testing.T) {
	sp := NewSupervisor("WaitFailed", []int{0, 100}, 3)
	sp.minRunningTimeMS = 100
	ctx := context.Background()

	count := 0
	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(nil, func() error {
			count++

			return errors.New("wait failed")
		})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 1, count)
}

func TestSupervisorRun_CacnelContext(t *testing.T) {
	sp := NewSupervisor("WaitFailed", []int{0, 100}, 3)
	sp.minRunningTimeMS = 100
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := sp.Run(ctx, func(ctx context.Context) Cmd {
		return newMockCmd(nil, func() error {
			count++

			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
			}

			return nil
		})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 1, count)
}
