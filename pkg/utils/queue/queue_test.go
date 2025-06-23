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

package queue

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
)

func TestHandleItemWithoutRetry(t *testing.T) {
	q := workqueue.NewNamed("q1")
	q.Add(t)
	q.Add("hello")
	q.Add(q)
	q.Add("hello") // drop duplicate
	q.Add("world")
	q.ShutDown()

	handler := func(ctx context.Context, key string) error {
		assert.Equal(t, key, "hello")
		return nil
	}

	assert.Equal(t, true, HandleQueueItemWithoutRetry(context.Background(), "TestQ", q, handler)) // t
	assert.Equal(t, true, q.ShuttingDown())
	assert.Equal(t, true, HandleQueueItemWithoutRetry(context.Background(), "TestQ", q, handler)) // hello
	assert.Equal(t, true, HandleQueueItemWithoutRetry(context.Background(), "TestQ", q, handler)) // q
	assert.Equal(t, true, HandleQueueItemWithoutRetry(context.Background(), "TestQ", q,
		func(ctx context.Context, key string) error {
			return fmt.Errorf("wrong")
		})) // world
	assert.Equal(t, q.Len(), 0)
	assert.Equal(t, true, !HandleQueueItemWithoutRetry(context.Background(), "TestQ", q, handler)) // shutdown
}

func TestCheckType(t *testing.T) {
	podItem := &PodQueueItem{Pod: &apicorev1.Pod{}}
	assert.Equal(t, "PodQueueItem", CheckType(podItem))

	nodeItem := &NodeQueueItem{Node: &apicorev1.Node{}}
	assert.Equal(t, "NodeQueueItem", CheckType(nodeItem))

	assert.Equal(t, "Unknown", CheckType("invalid type"))
	assert.Equal(t, "Unknown", CheckType(123))
	assert.Equal(t, "Unknown", CheckType(nil))
}
