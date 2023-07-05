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

func NewTestSessionManager() *SessionManager {
	config = &Config{
		TotalByteSizeLimit:         1024,
		PerSessionByteSizeLimit:    512,
		TopicQueueCapacity:         5,
		DeadSessionIDExpireSeconds: 6,
		SessionExpireSeconds:       4,
		CleanIntervalSeconds:       2,
		NormalizeActiveSeconds:     1,
	}
	return NewSessionManager()
}

func TestSessionManagerNormalPushAndPush(t *testing.T) {
	sm := NewTestSessionManager()

	// session queue full
	err := sm.Push("session1", "topic", NewMessageByRandomStr(520), time.Millisecond*100)
	assert.NotNil(t, err)

	err = sm.Push("session1", "topic", NewMessageByRandomStr(512), time.Millisecond*100)
	assert.Nil(t, err)

	err = sm.Push("session2", "topic", NewMessageByRandomStr(512), time.Millisecond*100)
	assert.Nil(t, err)
	assert.Equal(t, sm.memControl.totalByteSize, uint64(1024))

	// session manager full
	err = sm.Push("session3", "topic", NewMessageByRandomStr(512), time.Millisecond*100)
	assert.NotNil(t, err)

	msg, err := sm.Pop("session3", "topic", time.Millisecond*100)
	assert.Nil(t, msg)
	assert.Nil(t, err)

	msg, err = sm.Pop("session1", "topic", time.Millisecond*100)
	assert.NotNil(t, msg)
	assert.Nil(t, err)
	assert.Equal(t, sm.memControl.totalByteSize, uint64(512))

	msg, err = sm.Peek("session2", "topic")
	assert.NotNil(t, msg)
	assert.Nil(t, err)

	msg, err = sm.Peek("session2", "topic")
	assert.Nil(t, msg)
	assert.Nil(t, err)
}

func TestSessionManagerMemControl(t *testing.T) {
	sm := NewTestSessionManager()

	sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	sm.Push("session2", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	sm.Push("session3", "topic", NewMessageByRandomStr(24), time.Millisecond*100)

	err := sm.Push("session1", "topic", NewMessageByRandomStr(12), time.Millisecond*100)
	assert.NotNil(t, err)
	start := time.Now()
	go func() {
		time.Sleep(time.Millisecond * 500)
		sm.Peek("session2", "topic")
	}()

	err = sm.Push("session1", "topic", NewMessageByRandomStr(12), time.Second)
	processTime := time.Now().Sub(start)
	assert.True(t, processTime >= time.Millisecond*500)
	assert.Nil(t, err)
}

func TestWaitTwice(t *testing.T) {
	sm := NewTestSessionManager()

	sm.Push("session1", "topic", NewMessageByRandomStr(512), time.Millisecond*100)
	sm.Push("session2", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	sm.Push("session3", "topic", NewMessageByRandomStr(12), time.Millisecond*100)

	start := time.Now()
	go func() {
		time.Sleep(time.Millisecond * 300)
		sm.Peek("session2", "topic")
		time.Sleep(time.Millisecond * 200)
		sm.Peek("session1", "topic")
	}()

	err := sm.Push("session1", "topic", NewMessageByRandomStr(12), time.Millisecond*600)
	processTime := time.Now().Sub(start)
	assert.True(t, processTime >= time.Millisecond*500)
	assert.Nil(t, err)
}

func TestSessionExpired(t *testing.T) {
	sm := NewTestSessionManager()
	stopCh := make(chan struct{})
	sm.StartCleanLoop(stopCh)

	sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	time.Sleep(time.Second * 5)
	msg, err := sm.Peek("session1", "topic")
	assert.NotNil(t, err)
	assert.Nil(t, msg)
	err = sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100)
	assert.NotNil(t, err)

	sm.Push("session2", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	go func() {
		for true {
			select {
			case <-stopCh:
				break
			default:
				{
					time.Sleep(time.Second)
					_, err := sm.Peek("session2", "topic")
					assert.Nil(t, err)
				}
			}
		}
	}()
	time.Sleep(time.Second * 5)
	_, err = sm.Pop("session2", "topic", time.Millisecond*100)
	assert.Nil(t, err)
	close(stopCh)
}

func TestSessionDeadSessionID(t *testing.T) {
	sm := NewTestSessionManager()
	sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	sm.ReleaseSession("session1")
	err := sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100)
	assert.NotNil(t, err)

	stopCh := make(chan struct{})
	sm.StartCleanLoop(stopCh)
	time.Sleep(time.Second * 8)
	err = sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100)
	assert.Nil(t, err)
}

func TestRefreshSessionActiveMark(t *testing.T) {
	config.NormalizeActiveSeconds = 2
	sm := NewTestSessionManager()
	sm.Push("session1", "topic", NewMessageByRandomStr(10), time.Millisecond*100)
	time.Sleep(time.Second)
	sm.Push("session2", "topic", NewMessageByRandomStr(10), time.Millisecond*100)

	index1 := sm.sessions["session1"].ActiveMark.index
	time.Sleep(time.Second)
	sm.Pop("session1", "topic", time.Millisecond*100)
	index2 := sm.sessions["session1"].ActiveMark.index
	assert.NotEqual(t, index1, index2)

	sm.Pop("session2", "topic", time.Millisecond*100)
	index3 := sm.sessions["session1"].ActiveMark.index
	assert.Equal(t, index3, index2)
}

func TestReleaseTopicAndSession(t *testing.T) {
	sm := NewTestSessionManager()
	sm.Push("session1", "topic1", NewMessageByRandomStr(10), time.Millisecond*100)
	sm.Push("session1", "topic2", NewMessageByRandomStr(10), time.Millisecond*100)
	sm.Push("session1", "topic2", NewMessageByRandomStr(10), time.Millisecond*100)
	sm.Push("session2", "topic2", NewMessageByRandomStr(10), time.Millisecond*100)
	sm.ReleaseTopic("session1", "topic1")

	msg, err := sm.Peek("session1", "topic1")
	assert.Nil(t, msg)
	assert.Nil(t, err)
	assert.Equal(t, sm.memControl.totalByteSize, uint64(30))
	msg, err = sm.Peek("session1", "topic2")
	assert.NotNil(t, msg)
	assert.Nil(t, err)
	assert.Equal(t, sm.memControl.totalByteSize, uint64(20))

	sm.ReleaseSession("session1")
	assert.Equal(t, sm.memControl.totalByteSize, uint64(10))
	err = sm.Push("session1", "topic1", NewMessageByRandomStr(10), time.Millisecond*100)
	assert.NotNil(t, err)
}

func TestSessionReleasedDuringWait(t *testing.T) {
	sm := NewTestSessionManager()
	sm.Push("session1", "topic1", NewMessageByRandomStr(400), time.Millisecond*100)

	go func() {
		time.Sleep(time.Second)
		sm.ReleaseSession("session1")
	}()

	start := time.Now()
	err := sm.Push("session1", "topic1", NewMessageByRandomStr(200), time.Second*2)
	processTime := time.Now().Sub(start)

	assert.True(t, processTime > time.Second && processTime < time.Second*2)
	assert.NotNil(t, err)
	fmt.Println(err)

	sm.Push("session2", "topic1", NewMessageByRandomStr(100), time.Millisecond*100)
	sm.Peek("session2", "topic1")

	go func() {
		time.Sleep(time.Second)
		sm.ReleaseSession("session2")
	}()

	start = time.Now()
	_, err = sm.Pop("session2", "topic1", time.Second*2)
	processTime = time.Now().Sub(start)
	assert.True(t, processTime > time.Second && processTime < time.Second*2)
	assert.NotNil(t, err)
}
