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
	"sync"
	"time"

	"gitlab.com/jonas.jasas/condchan"

	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type SessionQueue struct {
	ByteSizeLimit uint64
	ByteSize      uint64

	released bool

	mtx      sync.Mutex
	notFull  *condchan.CondChan
	notEmpty *condchan.CondChan

	topics map[string]*Topic
}

func NewSessionQueue() *SessionQueue {
	sq := &SessionQueue{
		ByteSizeLimit: config.PerSessionByteSizeLimit,
		ByteSize:      0,
		released:      false,
		mtx:           sync.Mutex{},
		topics:        make(map[string]*Topic),
	}

	sq.notEmpty = condchan.New(&sq.mtx)
	sq.notFull = condchan.New(&sq.mtx)
	return sq
}

func (s *SessionQueue) Push(topic string, message *Message, timeout time.Duration) *transerr.TransError {
	if err := s.tryPush(topic, message, timeout); err != nil {
		return err
	}

	s.notEmpty.Signal()
	return nil
}

func (s *SessionQueue) Pop(topic string, timeout time.Duration) (*Message, *transerr.TransError) {
	message, err := s.tryPop(topic, timeout)
	if err != nil || message == nil {
		return message, err
	}

	s.notFull.Signal()
	return message, err
}

func (s *SessionQueue) Peek(topic string) (*Message, *transerr.TransError) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.released {
		return nil, transerr.NewTransError(transerr.SessionReleased)
	}

	if s.availableToPop(topic) {
		message := s.innerPop(topic)
		if message != nil {
			s.notFull.Signal()
			return message, nil
		}
	}
	return nil, nil
}

func (s *SessionQueue) ReleaseTopic(topic string) uint64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.released {
		return 0
	}
	topicQueue, ok := s.topics[topic]
	if !ok {
		return 0
	}

	s.ByteSize -= topicQueue.ByteSize
	delete(s.topics, topic)

	s.notFull.Signal()
	return topicQueue.ByteSize
}

func (s *SessionQueue) ReleaseSession() uint64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.released = true
	s.topics = nil
	byteSize := s.ByteSize
	s.ByteSize = 0

	s.notFull.Signal()
	s.notEmpty.Signal() // notify pop failed
	return byteSize
}

// getTopic not thread safe
func (s *SessionQueue) getTopic(topic string) *Topic {
	topicQueue, exists := s.topics[topic]
	if exists {
		return topicQueue
	}

	topicQueue = NewTopicQueue(topic)
	s.topics[topic] = topicQueue
	return topicQueue
}

func (s *SessionQueue) waitUntil(check func() bool, cond *condchan.CondChan, timeout time.Duration) (bool, *transerr.TransError) {
	if s.released {
		return false, transerr.NewTransError(transerr.SessionReleased)
	}

	if check() {
		return true, nil
	}

	timeoutChan := time.After(timeout)
	waitTimeout := false
	available := false
	for {
		cond.Select(func(c <-chan struct{}) {
			select {
			case <-c:
			case <-timeoutChan:
				waitTimeout = true
			}
		})
		if s.released || waitTimeout {
			break
		}

		if available = check(); available {
			break
		}
	}

	if s.released {
		return false, transerr.NewTransError(transerr.SessionReleased)
	}

	return available, nil
}

func (s *SessionQueue) tryPush(topic string, message *Message, timeout time.Duration) *transerr.TransError {
	if message.ByteSize() > s.ByteSizeLimit {
		nlog.Warnf("session queue topic(%s) new message len(%d) max than total buffer size(%d)",
			topic, message.ByteSize(), s.ByteSizeLimit)
		return transerr.NewTransError(transerr.BufferOverflow)
	}
	checkFn := func() bool {
		return s.availableToPush(message)
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	available, err := s.waitUntil(checkFn, s.notFull, timeout)
	if err != nil {
		return err
	}
	if !available {
		nlog.Infof("not found available buffer for topic(%s), len(%d)", topic, message.ByteSize())
		return transerr.NewTransError(transerr.BufferOverflow)
	}
	s.innerPush(topic, message)
	return nil
}

func (s *SessionQueue) tryPop(topic string, timeout time.Duration) (*Message, *transerr.TransError) {
	checkFn := func() bool {
		return s.availableToPop(topic)
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	available, err := s.waitUntil(checkFn, s.notEmpty, timeout)
	if err != nil || !available {
		return nil, err
	}

	return s.innerPop(topic), nil
}

func (s *SessionQueue) availableToPush(message *Message) bool {
	return s.ByteSize+message.ByteSize() <= s.ByteSizeLimit
}

func (s *SessionQueue) availableToPop(topic string) bool {
	topicQueue, ok := s.topics[topic]
	return ok && topicQueue.Len() != 0
}

func (s *SessionQueue) innerPush(topic string, message *Message) {
	topicQueue := s.getTopic(topic)
	topicQueue.Push(message)
	s.ByteSize += message.ByteSize()
}

func (s *SessionQueue) innerPop(topic string) *Message {
	topicQueue := s.getTopic(topic)
	message := topicQueue.Pop()

	if message != nil {
		s.ByteSize -= message.ByteSize()
	}
	return message
}
