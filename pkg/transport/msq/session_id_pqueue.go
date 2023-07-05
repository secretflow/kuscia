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

type SessionIDItem struct {
	sid       string
	index     int
	timestamp int64 // priority
}

func NewSessionIDItem(id string, t int64) *SessionIDItem {
	return &SessionIDItem{
		sid:       id,
		index:     -1,
		timestamp: t,
	}
}

type SessionIDPQ []*SessionIDItem

func (pq *SessionIDPQ) Len() int {
	return len(*pq)
}

func (pq *SessionIDPQ) Less(i, j int) bool {
	// the oldest timestamp has highest priority
	return (*pq)[i].timestamp < (*pq)[j].timestamp
}

func (pq *SessionIDPQ) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index, (*pq)[j].index = i, j
}

func (pq *SessionIDPQ) Push(x interface{}) {
	item := x.(*SessionIDItem)
	item.index = pq.Len()
	*pq = append(*pq, item)
}

func (pq *SessionIDPQ) Pop() interface{} {
	old := *pq
	n := pq.Len()
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
