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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestDeadSessionID(t *testing.T) {
	config := &Config{
		DeadSessionIDExpireSeconds: 1,
	}
	deadSessionID := NewDeadSessionID(config)
	stopCh := make(chan struct{})
	go func() {
		go wait.Until(deadSessionID.Clean, time.Millisecond*100, stopCh)
	}()

	deadSessionID.Push("sid1")
	assert.True(t, deadSessionID.Exists("sid1"))
	assert.False(t, deadSessionID.Exists("sid2"))

	time.Sleep(time.Second * 2)
	deadSessionID.Push("sid2")
	time.Sleep(time.Second * 2)
	deadSessionID.Push("sid3")
	time.Sleep(time.Millisecond * 300)

	assert.True(t, deadSessionID.Exists("sid3"))
	assert.False(t, deadSessionID.Exists("sid1"))
	assert.False(t, deadSessionID.Exists("sid2"))
	close(stopCh)
}
