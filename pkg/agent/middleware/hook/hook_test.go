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

func (p *mockHookHandlerA) CanExec(ctx Context) bool {
	return true
}

func (p *mockHookHandlerA) ExecHook(ctx Context) (*Result, error) {
	if ctx.Point() == PointMakeMounts {
		return &Result{Terminated: true, Msg: "terminated by mock"}, nil
	}
	return &Result{}, nil
}

func TestExecute(t *testing.T) {
	Register("mock-handler-a", &mockHookHandlerA{})

	err := Execute(&K8sProviderSyncPodContext{})
	assert.NoError(t, err)

	err = Execute(&MakeMountsContext{})
	assert.ErrorContains(t, err, "terminate operation")
}
