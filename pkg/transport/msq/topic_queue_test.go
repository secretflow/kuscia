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
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func NewMessageByStr(str string) *Message {
	content := []byte(str)
	return NewMessage(content)
}

func NewMessageByRandomStr(l int) *Message {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	content := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		content = append(content, bytes[r.Intn(len(bytes))])
	}
	return NewMessage(content)
}

func TestTopicQueue(t *testing.T) {
	queue := NewTopicQueue("topic1")
	msg1 := NewMessageByStr("hi")
	msg2 := NewMessageByStr("world")
	queue.Push(msg1)
	queue.Push(msg2)

	assert.Equal(t, queue.ByteSize, (uint64)(7))
	assert.Equal(t, queue.Len(), 2)

	msg3 := queue.Pop()
	assert.True(t, string(msg3.Content) == "hi")
	assert.Equal(t, queue.ByteSize, (uint64)(5))

	msg4 := queue.Pop()
	assert.True(t, string(msg4.Content) == "world")

	msg5 := queue.Pop()
	assert.Nil(t, msg5)

	assert.Equal(t, queue.ByteSize, (uint64)(0))
}
