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

package lock

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

func TestKeyLocker(t *testing.T) {
	t.Parallel()
	kl := NewKeyLocker()

	totalCase := 20
	threadPerCase := runtime.NumCPU() * 3

	wg := sync.WaitGroup{}
	sum := make([]int64, totalCase)
	wg.Add(totalCase * threadPerCase)

	for cases := 0; cases < totalCase; cases++ {
		key := fmt.Sprintf("key%v", cases)

		for ths := 0; ths < threadPerCase; ths++ {
			go func(cas int) {
				defer wg.Done()

				for i := 0; i < 200; i++ {
					kl.Lock(key)
					sum[cas] = sum[cas] + 1
					kl.Unlock(key)
				}
			}(cases)
		}
	}

	wg.Wait()
	for i := 0; i < totalCase; i++ {
		assert.Equal(t, sum[i], int64(200*threadPerCase))
	}
	assert.Equal(t, len(kl.inUse), 0)
}
