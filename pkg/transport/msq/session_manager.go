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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/secretflow/kuscia/pkg/transport/transerr"
)

type Session struct {
	Queue      *SessionQueue
	ActiveMark *SessionIDItem
}

type SessionManager struct {
	sync.RWMutex

	sessions           map[string]*Session
	deadSessionIDs     *DeadSessionID
	activeSessionIDs   SessionIDPQ
	normalizeTimeScale time.Duration
	memControl         *MemControl
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		RWMutex:        sync.RWMutex{},
		sessions:       make(map[string]*Session),
		deadSessionIDs: NewDeadSessionID(config),
		memControl:     NewMemControl(config),
	}
}

func (s *SessionManager) StartCleanLoop(stopCh <-chan struct{}) {
	round := 0
	cleanFn := func() {
		if round%2 == 0 {
			s.cleanInactiveSession()
		} else {
			s.deadSessionIDs.Clean()
		}
		round++
	}
	cleanInterval := time.Duration(config.CleanIntervalSeconds>>1) * time.Second
	go wait.Until(cleanFn, cleanInterval, stopCh)
}

func (s *SessionManager) Push(sid, topic string, message *Message, timeout time.Duration) *transerr.TransError {
	sq, err := s.GetOrCreateSession(sid, false)
	if err != nil {
		return err
	}

	ok, leftTime := s.memControl.Prefetch(message.ByteSize(), timeout)
	if !ok {
		return transerr.NewTransError(transerr.BufferOverflow)
	}

	err = sq.Push(topic, message, leftTime)
	if err != nil {
		s.memControl.Release(message.ByteSize())
	}
	return err
}

func (s *SessionManager) Pop(sid, topic string, timeout time.Duration) (*Message, *transerr.TransError) {
	sq, err := s.GetOrCreateSession(sid, true)
	if err != nil {
		return nil, err
	}

	message, err := sq.Pop(topic, timeout)
	if message != nil {
		s.memControl.Release(message.ByteSize())
	}
	return message, err
}

func (s *SessionManager) Peek(sid, topic string) (*Message, *transerr.TransError) {
	sq, err := s.GetOrCreateSession(sid, true)
	if err != nil {
		return nil, err
	}

	message, err := sq.Peek(topic)
	if message != nil {
		s.memControl.Release(message.ByteSize())
	}
	return message, err
}

func (s *SessionManager) ReleaseTopic(sid, topic string) {
	sq, _ := s.GetSession(sid, false)
	if sq == nil {
		return
	}

	if topicByteSize := sq.ReleaseTopic(topic); topicByteSize > 0 {
		s.memControl.Release(topicByteSize)
	}
}

func (s *SessionManager) ReleaseSession(sid string) {
	sq, _ := s.GetSession(sid, false)
	if sq == nil {
		return
	}

	// delete session from session map before release session
	s.deleteSessionQueue(sid)

	if sessionByteSize := sq.ReleaseSession(); sessionByteSize > 0 {
		s.memControl.Release(sessionByteSize)
	}
}

func (s *SessionManager) GetSession(sid string, refresh bool) (*SessionQueue, *transerr.TransError) {
	sq, verifyRefresh := s.getSessionAndVerifyRefresh(sid, refresh)
	if sq == nil || !verifyRefresh {
		return sq, nil
	}

	s.Lock()
	defer s.Unlock()
	session := s.sessions[sid]
	if session == nil {
		return nil, transerr.NewTransError(transerr.SessionReleased)
	}
	session.ActiveMark.timestamp = s.normalizedNowTimestamp()
	heap.Fix(&s.activeSessionIDs, session.ActiveMark.index)
	return session.Queue, nil
}

func (s *SessionManager) GetOrCreateSession(sid string, refresh bool) (*SessionQueue, *transerr.TransError) {
	sessionQueue, err := s.GetSession(sid, refresh)
	if sessionQueue != nil || err != nil {
		return sessionQueue, err
	}

	return s.createSessionQueue(sid)
}

func (s *SessionManager) cleanInactiveSession() {
	currentTimestamp := s.normalizedNowTimestamp()
	expireDuration := config.SessionExpireSeconds / config.NormalizeActiveSeconds
	inactiveQueues := make([]*SessionQueue, 0)

	s.Lock()
	count := min(s.activeSessionIDs.Len(), cleanStep)
	for i := 0; i < count; i++ {
		item := s.activeSessionIDs[0]
		if currentTimestamp-item.timestamp < expireDuration {
			break
		}

		heap.Pop(&s.activeSessionIDs)
		session := s.sessions[item.sid]
		// if a session has been released before, the session here maybe nil
		if session != nil {
			s.deadSessionIDs.Push(item.sid)
			inactiveQueues = append(inactiveQueues, session.Queue)
		}
		delete(s.sessions, item.sid)
	}
	s.Unlock()

	for _, sq := range inactiveQueues {
		if sessionByteSize := sq.ReleaseSession(); sessionByteSize > 0 {
			s.memControl.Release(sessionByteSize)
		}
	}
}

func (s *SessionManager) getSessionAndVerifyRefresh(sid string, refresh bool) (*SessionQueue, bool) {
	s.RLock()
	defer s.RUnlock()
	session, ok := s.sessions[sid]
	if !ok {
		return nil, refresh
	}

	if refresh {
		now := s.normalizedNowTimestamp()
		if now > session.ActiveMark.timestamp {
			refresh = true
		}
	}
	return session.Queue, refresh
}

func (s *SessionManager) createSessionQueue(sid string) (*SessionQueue, *transerr.TransError) {
	s.Lock()
	defer s.Unlock()
	if ss := s.sessions[sid]; ss != nil {
		return ss.Queue, nil
	}

	if s.deadSessionIDs.Exists(sid) {
		return nil, transerr.NewTransError(transerr.SessionReleased)
	}

	sessionQueue := NewSessionQueue()
	SessionIDItem := NewSessionIDItem(sid, s.normalizedNowTimestamp())
	session := &Session{
		Queue:      sessionQueue,
		ActiveMark: SessionIDItem,
	}

	heap.Push(&s.activeSessionIDs, SessionIDItem)
	if SessionIDItem.index == -1 {
		panic("Invalid SessionIDItem")
	}
	s.sessions[sid] = session
	return sessionQueue, nil
}

func (s *SessionManager) deleteSessionQueue(sid string) {
	s.Lock()
	defer s.Unlock()

	s.deadSessionIDs.Push(sid)

	// here do not remove sid from activeSessionIDs, just depend on cleanLoop
	delete(s.sessions, sid)
}

func (s *SessionManager) normalizedNowTimestamp() int64 {
	return time.Now().Unix() / config.NormalizeActiveSeconds
}
