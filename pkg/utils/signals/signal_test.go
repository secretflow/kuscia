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

package signals

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewKusciaContextWithStopCh(t *testing.T) {
	stopCh := make(chan struct{})
	ctx := NewKusciaContextWithStopCh(stopCh)
	go func() {
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()
	err := ctx.Err()
	assert.NoError(t, err)
	<-ctx.Done()
	_, ok := ctx.Deadline()
	assert.Equal(t, ok, false)
	err = ctx.Err()
	assert.Equal(t, err.Error(), "receive shutdown signals")
}

func Test_SetupSignalHandler(t *testing.T) {
	SetupSignalHandler()
}
