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
	"container/heap"
	"time"
)

type DeadSessionID struct {
	expiredMs int64

	sids map[string]struct{}
	pq   SessionIDPQ
}

func NewDeadSessionID(config *Config) *DeadSessionID {
	return &DeadSessionID{
		expiredMs: config.DeadSessionIDExpireSeconds * 1000,
		sids:      make(map[string]struct{}),
		pq:        make(SessionIDPQ, 0, sidPqInitialCapacity),
	}
}

func (ds *DeadSessionID) Push(sid string) {
	_, ok := ds.sids[sid]
	if ok {
		return
	}

	ds.sids[sid] = struct{}{}
	SessionIDItem := NewSessionIDItem(sid, nowMs())
	heap.Push(&ds.pq, SessionIDItem)
}

func (ds *DeadSessionID) Exists(sid string) bool {
	_, ok := ds.sids[sid]
	return ok
}

func (ds *DeadSessionID) Clean() {
	currentMs := nowMs()
	count := min(ds.pq.Len(), cleanStep)
	for i := 0; i < count; i++ {
		item := ds.pq[0]
		if currentMs-item.timestamp < ds.expiredMs {
			break
		}

		heap.Pop(&ds.pq)
		delete(ds.sids, item.sid)
	}
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}
