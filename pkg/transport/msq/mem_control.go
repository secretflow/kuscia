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

package msq

import (
	"context"
	"sync"
	"time"

	"github.com/Jille/contextcond"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type MemControl struct {
	totalByteSizeLimit uint64
	totalByteSize      uint64

	sync.Mutex

	notFull *contextcond.Cond
}

func NewMemControl(config *Config) *MemControl {
	mc := &MemControl{
		totalByteSizeLimit: config.TotalByteSizeLimit,
		totalByteSize:      0,
		Mutex:              sync.Mutex{},
	}
	mc.notFull = contextcond.NewCond(&mc.Mutex)
	return mc
}

func (mc *MemControl) Prefetch(byteSize uint64, timeout time.Duration) (bool, time.Duration) {
	leftTimeout := timeout
	mc.Lock()
	defer mc.Unlock()
	available := mc.availableToPush(byteSize)
	if !available {
		if byteSize > mc.totalByteSizeLimit {
			nlog.Warnf("input body size(%d) max than maxByteLimit(%d), so skip it", byteSize, mc.totalByteSizeLimit)
			return false, leftTimeout
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		for {
			if mc.notFull.WaitContext(ctx) != nil {
				break
			}

			if available = mc.availableToPush(byteSize); available {
				usedTime := time.Since(start)
				if leftTimeout > usedTime {
					leftTimeout -= usedTime
				} else {
					leftTimeout = 0
				}
				break
			}
		}
	}

	if !available || leftTimeout == 0 {
		return false, leftTimeout
	}
	mc.totalByteSize += byteSize
	return true, leftTimeout
}

func (mc *MemControl) Release(byteSize uint64) {
	mc.Lock()
	mc.totalByteSize -= byteSize
	mc.Unlock()

	mc.notFull.Signal()
}

func (mc *MemControl) availableToPush(byteSize uint64) bool {
	return mc.totalByteSize+byteSize <= mc.totalByteSizeLimit
}
