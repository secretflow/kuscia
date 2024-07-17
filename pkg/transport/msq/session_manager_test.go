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
	"k8s.io/apimachinery/pkg/util/wait"
)

func NewTestSessionManager() *SessionManager {
	config := &Config{
		TotalByteSizeLimit:         1024,
		PerSessionByteSizeLimit:    512,
		TopicQueueCapacity:         5,
		DeadSessionIDExpireSeconds: 6,
		SessionExpireSeconds:       4,
		CleanIntervalSeconds:       2,
		NormalizeActiveSeconds:     1,
	}
	return NewSessionManager(config)
}

func TestSessionManagerNormalPushAndPush(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	processTime := time.Since(start)
	assert.True(t, processTime >= time.Millisecond*500)
	assert.Nil(t, err)
}

func TestWaitTwice(t *testing.T) {
	t.Parallel()
	sm := NewTestSessionManager()

	assert.Nil(t, sm.Push("session1", "topic", NewMessageByRandomStr(512), time.Millisecond*100))
	assert.Nil(t, sm.Push("session2", "topic", NewMessageByRandomStr(500), time.Millisecond*100))
	assert.Nil(t, sm.Push("session3", "topic", NewMessageByRandomStr(12), time.Millisecond*100))

	// total buffer is full, so return error
	assert.NotNil(t, sm.Push("session1", "topic", NewMessageByRandomStr(12), time.Millisecond*20))

	// session1's buffer is full, so return error
	sm.Peek("session3", "topic")
	assert.NotNil(t, sm.Push("session1", "topic", NewMessageByRandomStr(12), time.Millisecond*20))

	// clean session1
	sm.Peek("session1", "topic")
	start := time.Now()
	assert.Nil(t, sm.Push("session1", "topic", NewMessageByRandomStr(12), 2*time.Second))

	processTime := time.Since(start)
	assert.Less(t, processTime, time.Second) // normally less than 10ms, but test machine may run slowly
}

func getQuickExpireConfig() *Config {
	return &Config{
		TotalByteSizeLimit:         1024,
		PerSessionByteSizeLimit:    512,
		TopicQueueCapacity:         5,
		DeadSessionIDExpireSeconds: 1,
		SessionExpireSeconds:       1,
		CleanIntervalSeconds:       1,
		NormalizeActiveSeconds:     1,
	}

}

func TestSessionExpired_Inactivate(t *testing.T) {
	t.Parallel()
	sm := NewSessionManager(getQuickExpireConfig())

	assert.Nil(t, sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100))
	assert.Equal(t, 0, len(sm.deadSessionIDs.sids))

	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second*2, func() (bool, error) {
		sm.cleanInactiveSession()
		return len(sm.deadSessionIDs.sids) != 0, nil
	}))

	assert.Contains(t, sm.deadSessionIDs.sids, "session1")
	assert.NotContains(t, sm.sessions, "session1")
}

func TestSessionExpired(t *testing.T) {
	t.Parallel()
	sm := NewSessionManager(getQuickExpireConfig())
	stopCh := make(chan struct{})
	sm.StartCleanLoop(stopCh)

	sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100)
	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second*3, func() (bool, error) {
		sm.Lock()
		defer sm.Unlock()
		_, ok := sm.sessions["session1"]
		return !ok, nil
	}))

	// session1 is dead
	msg, err := sm.Peek("session1", "topic")
	assert.NotNil(t, err)
	assert.Nil(t, msg)

	// session is dead, but not cleaned, so push return error
	assert.NotNil(t, sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100))

	close(stopCh)
}

func TestSessionExpired_KeepAlive(t *testing.T) {
	t.Parallel()
	config := getQuickExpireConfig()
	config.SessionExpireSeconds = 2
	sm := NewSessionManager(config)
	stopCh := make(chan struct{})
	sm.StartCleanLoop(stopCh)

	assert.Nil(t, sm.Push("session2", "topic", NewMessageByRandomStr(500), time.Millisecond*100))
	go wait.Until(func() {
		_, err := sm.Peek("session2", "topic")
		assert.Nil(t, err)
	}, time.Millisecond*200, stopCh)

	for i := 0; i < 4; i++ {
		_, ok := sm.sessions["session2"]
		assert.True(t, ok) // session still exists
		time.Sleep(time.Millisecond * 500)
	}

	close(stopCh)
}

func TestSessionDeadSessionID(t *testing.T) {
	t.Parallel()
	sm := NewSessionManager(getQuickExpireConfig())
	assert.Nil(t, sm.Push("session1", "topic", NewMessageByRandomStr(500), time.Millisecond*100))
	sm.ReleaseSession("session1")
	assert.NotNil(t, sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100))

	stopCh := make(chan struct{})
	sm.StartCleanLoop(stopCh)

	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second*5, func() (bool, error) {
		sm.Lock()
		defer sm.Unlock()
		_, sExists := sm.sessions["session1"]
		_, iExists := sm.deadSessionIDs.sids["session1"]
		assert.False(t, sExists && iExists) // session can't both in sessions and deadsessions
		return !(sExists || iExists), nil
	}))

	// sesssion is dead, so can push message again
	assert.Nil(t, sm.Push("session1", "topic", NewMessageByRandomStr(5), time.Millisecond*100))
}

func TestRefreshSessionActiveMark(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	sm := NewTestSessionManager()
	sm.Push("session1", "topic1", NewMessageByRandomStr(400), time.Millisecond*100)

	go func() {
		time.Sleep(time.Second)
		sm.ReleaseSession("session1")
	}()

	start := time.Now()
	err := sm.Push("session1", "topic1", NewMessageByRandomStr(200), time.Second*2)
	processTime := time.Since(start)

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
	processTime = time.Since(start)
	assert.True(t, processTime > time.Second && processTime < time.Second*2)
	assert.NotNil(t, err)
}
