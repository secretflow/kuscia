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
	"sync"
)

type refCounter struct {
	counter int64
	lock    *sync.Mutex
}

type KeyLocker struct {
	globalLock sync.Mutex

	inUse map[string]*refCounter
	pool  *sync.Pool
}

func (kl *KeyLocker) Lock(key string) {
	kl.globalLock.Lock()
	m := kl.getLocker(key)
	m.counter++
	kl.globalLock.Unlock()

	m.lock.Lock()
}

func (kl *KeyLocker) Unlock(key string) {
	kl.globalLock.Lock()
	defer kl.globalLock.Unlock()

	m := kl.getLocker(key)
	m.counter--
	m.lock.Unlock()
	if m.counter == 0 {
		delete(kl.inUse, key)
		kl.pool.Put(m)
	}
}

func (kl *KeyLocker) getLocker(key string) *refCounter {
	if rc, exist := kl.inUse[key]; exist {
		return rc
	}

	rc := kl.pool.Get().(*refCounter)
	kl.inUse[key] = rc
	return rc
}

// NewKeyLocker create a new locker
func NewKeyLocker() *KeyLocker {
	return &KeyLocker{
		inUse: map[string]*refCounter{},
		pool: &sync.Pool{
			New: func() interface{} {
				return &refCounter{
					counter: 0,
					lock:    &sync.Mutex{},
				}
			},
		},
	}
}
