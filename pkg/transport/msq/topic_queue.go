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

type Message struct {
	Content []byte
}

type Topic struct {
	ByteSize uint64
	queue    []*Message
}

func NewMessage(msg []byte) *Message {
	return &Message{
		Content: msg,
	}
}

func NewTopicQueue(topic string) *Topic {
	return &Topic{
		ByteSize: 0,
		queue:    make([]*Message, 0, config.TopicQueueCapacity),
	}
}

func (t *Topic) Push(message *Message) {
	t.ByteSize += message.ByteSize()
	t.queue = append(t.queue, message)
}

func (t *Topic) Pop() *Message {
	if len(t.queue) == 0 {
		return nil
	}

	message := t.queue[0]
	t.queue[0] = nil
	t.queue = t.queue[1:]

	t.ByteSize -= message.ByteSize()
	return message
}

func (t *Topic) Len() int {
	return len(t.queue)
}

func (m *Message) ByteSize() uint64 {
	return uint64(len(m.Content))
}
