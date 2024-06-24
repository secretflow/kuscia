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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func initConfig() {
	config = &Config{
		TotalByteSizeLimit:         1024 * 1024,
		PerSessionByteSizeLimit:    1024,
		TopicQueueCapacity:         5,
		DeadSessionIDExpireSeconds: 1800,
		SessionExpireSeconds:       4,
		CleanIntervalSeconds:       2,
		NormalizeActiveSeconds:     1,
	}
}

func NewTestSessionQueue() *SessionQueue {
	initConfig()
	sq := NewSessionQueue()
	return sq
}

func TestSessionQueuePushNoWait(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 20480

	start := time.Now()
	for i := 0; i < 1000; i++ {
		assert.Nil(t, sq.Push("topic", NewMessageByStr("12"), time.Second))
	}
	processTime := time.Since(start)
	assert.Less(t, processTime, time.Millisecond*500)
}

func TestSessionQueuePushTimeout(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 100

	start := time.Now()
	msg := NewMessageByRandomStr(101)
	err := sq.Push("topic", msg, time.Second*5)
	assert.NotNil(t, err)
	processTime := time.Since(start)
	assert.Less(t, processTime, time.Millisecond*100)
}

func producer(sq *SessionQueue, t *testing.T) {
	for i := 0; i < 10; i++ {
		err := sq.Push("topic", NewMessageByRandomStr(55), time.Second)
		assert.Nil(t, err)
	}
}

func TestSessionQueuePushWait(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 100

	start := time.Now()
	doneCh := make(chan struct{})

	// consumer
	go func() {
		for i := 0; i < 20; i++ {
			msg, err := sq.Pop("topic", time.Millisecond*200)
			assert.NotNil(t, msg)
			assert.Nil(t, err)
			time.Sleep(time.Millisecond * 20)
		}
		close(doneCh)
	}()

	go producer(sq, t)
	go producer(sq, t)

	<-doneCh
	processTime := time.Since(start)
	assert.Less(t, processTime, time.Second*5)
}

func TestSessionQueuePopNoWait(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 20480

	for i := 0; i < 1000; i++ {
		sq.Push("topic", NewMessageByStr("12"), time.Minute)
	}
	start := time.Now()
	for i := 0; i < 1000; i++ {
		msg, err := sq.Pop("topic", time.Minute)
		assert.NotNil(t, msg)
		assert.Nil(t, err)
	}
	processTime := time.Since(start)
	assert.Less(t, processTime, time.Second*5)
}

func TestSessionQueuePopTimeout(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 20480

	start := time.Now()
	msg, err := sq.Pop("topic", time.Second)
	assert.Nil(t, msg)
	assert.Nil(t, err)
	processTime := time.Since(start)
	assert.Greater(t, processTime, time.Second)
}

func TestSessionQueuePopWait(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 20480

	pushedCount := 0
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Millisecond * 10)
			err := sq.Push("topic", NewMessageByStr(fmt.Sprintf("%d", i)), time.Second)
			assert.Nil(t, err)
			pushedCount++
		}
	}()

	for i := 0; i < 10; i++ {
		start := time.Now()
		msg, err := sq.Pop("topic", time.Millisecond*500)
		assert.NotNil(t, msg)
		assert.Nil(t, err)
		assert.Equal(t, fmt.Sprintf("%d", i), string(msg.Content))

		processTime := time.Since(start)
		assert.Greater(t, processTime, time.Millisecond)
	}

	assert.Equal(t, 10, pushedCount)
}

func TestSessionQueuePeek(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 20480

	doneCh := make(chan struct{})
	for i := 0; i < 10; i++ {
		msg, err := sq.Peek("topic")
		assert.Nil(t, msg)
		assert.Nil(t, err)
	}

	go func() {
		for i := 0; i < 10; i++ {
			assert.Nil(t, sq.Push("topic", NewMessageByStr(fmt.Sprintf("%d", i)), time.Second))
		}
		close(doneCh)
	}()

	<-doneCh

	for i := 0; i < 10; i++ {
		msg, err := sq.Peek("topic")
		assert.NotNil(t, msg)
		assert.Equal(t, fmt.Sprintf("%d", i), string(msg.Content))
		assert.Nil(t, err)
	}
}

func TestSessionQueueMultiTopic(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 9
	err := sq.Push("topic1", NewMessageByStr("hello"), time.Millisecond*100)
	assert.Nil(t, err)

	err = sq.Push("topic2", NewMessageByStr("world"), time.Millisecond*100)
	assert.NotNil(t, err)

	msg, err := sq.Peek("topic1")
	assert.Equal(t, "hello", string(msg.Content))
	assert.Nil(t, err)
	_, err = sq.Peek("topic2")
	assert.Nil(t, err)

	err = sq.Push("topic2", NewMessageByStr("world"), time.Millisecond*100)
	assert.Nil(t, err)
	msg, err = sq.Peek("topic2")
	assert.Equal(t, "world", string(msg.Content))
	assert.Nil(t, err)

	err = sq.Push("topic1", NewMessageByStr("hello"), time.Millisecond*100)
	assert.Nil(t, err)

	go func() {
		time.Sleep(time.Millisecond * 200)
		sq.Peek("topic1")
	}()
	err = sq.Push("topic3", NewMessageByStr("world"), time.Millisecond*300)
	assert.Nil(t, err)
}

func TestSessionQueueReleaseTopic(t *testing.T) {
	t.Parallel()
	sq := NewTestSessionQueue()
	sq.ByteSizeLimit = 100
	sq.Push("topic1", NewMessageByStr("hello"), time.Millisecond*100)
	sq.Push("topic1", NewMessageByStr("world"), time.Millisecond*100)
	sq.Push("topic2", NewMessageByStr("hi"), time.Millisecond*100)
	assert.Equal(t, sq.ByteSize, uint64(12))
	sq.ReleaseTopic("topic1")
	assert.Equal(t, sq.ByteSize, uint64(2))

	msg, _ := sq.Peek("topic1")
	assert.Nil(t, msg)
}
