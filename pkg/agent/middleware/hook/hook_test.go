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

package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockHookHandlerA struct {
}

func (p *mockHookHandlerA) CanExec(obj interface{}, point Point) bool {
	return point == 0
}

func (p *mockHookHandlerA) ExecHook(obj interface{}, point Point) (*Result, error) {
	if obj == nil {
		return &Result{Terminated: true, Msg: "terminated by mock"}, nil
	}
	return &Result{}, nil
}

func TestExecute(t *testing.T) {
	Register("mock-handler-a", &mockHookHandlerA{})

	err := Execute(new(int), 0)
	assert.NoError(t, err)

	err = Execute(nil, 0)
	assert.ErrorContains(t, err, "terminate operation")
}
