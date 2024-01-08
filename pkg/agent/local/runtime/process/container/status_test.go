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

package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func TestContainerState(t *testing.T) {
	for c, test := range map[string]struct {
		status Status
		state  runtime.ContainerState
	}{
		"unknown state": {
			status: Status{
				Unknown: true,
			},
			state: runtime.ContainerState_CONTAINER_UNKNOWN,
		},
		"unknown state because there is no timestamp set": {
			status: Status{},
			state:  runtime.ContainerState_CONTAINER_UNKNOWN,
		},
		"created state": {
			status: Status{
				CreatedAt: time.Now().UnixNano(),
			},
			state: runtime.ContainerState_CONTAINER_CREATED,
		},
		"running state": {
			status: Status{
				CreatedAt: time.Now().UnixNano(),
				StartedAt: time.Now().UnixNano(),
			},
			state: runtime.ContainerState_CONTAINER_RUNNING,
		},
		"exited state": {
			status: Status{
				CreatedAt:  time.Now().UnixNano(),
				FinishedAt: time.Now().UnixNano(),
			},
			state: runtime.ContainerState_CONTAINER_EXITED,
		},
	} {
		t.Logf("TestCase %q", c)
		assert.Equal(t, test.state, test.status.State())
	}
}
