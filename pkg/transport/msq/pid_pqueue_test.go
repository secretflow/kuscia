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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionIDPq(t *testing.T) {
	pq := make(SessionIDPQ, 0, 5)
	heap.Init(&pq)
	heap.Push(&pq, &SessionIDItem{sid: "1", timestamp: 10})
	heap.Push(&pq, &SessionIDItem{sid: "2", timestamp: 30})
	heap.Push(&pq, &SessionIDItem{sid: "3", timestamp: 20})

	assert.Equal(t, pq[0].sid, "1")
	heap.Pop(&pq)

	assert.Equal(t, pq[0].sid, "3")
	heap.Pop(&pq)

	assert.Equal(t, pq[0].sid, "2")
	heap.Pop(&pq)

	assert.Equal(t, pq.Len(), 0)
}

func TestUpdateSessionIDPriority(t *testing.T) {
	pq := make(SessionIDPQ, 0, 5)
	heap.Init(&pq)

	item1 := &SessionIDItem{sid: "1", timestamp: 10}
	item2 := &SessionIDItem{sid: "2", timestamp: 30}
	item3 := &SessionIDItem{sid: "2", timestamp: 20}
	heap.Push(&pq, item1)
	heap.Push(&pq, item2)
	heap.Push(&pq, item3)

	assert.Equal(t, item1.index, 0)
	item1.timestamp = 40
	heap.Fix(&pq, item1.index)
	assert.NotEqual(t, item1.index, 0)
}
