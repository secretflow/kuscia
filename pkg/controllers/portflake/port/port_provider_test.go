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

package port

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestProvider_Allocate(t *testing.T) {
	pp := newProvider("ns_a")

	totalPortCount := pp.segments.Count()

	portsPerBatch := 10
	successCount := totalPortCount / portsPerBatch

	for i := 0; i < successCount; i++ {
		ports, err := pp.Allocate(portsPerBatch)
		assert.NilError(t, err)
		assert.Equal(t, portsPerBatch, len(ports), "Allocate No.%d", i)
	}
	assert.Equal(t, successCount*portsPerBatch, pp.PortToVerifyCount())
	assert.Equal(t, successCount*portsPerBatch, pp.PortCount())

	_, err := pp.Allocate(portsPerBatch)
	assert.ErrorContains(t, err, "no available port to allocate")
	assert.Equal(t, successCount*portsPerBatch, pp.PortToVerifyCount())
	assert.Equal(t, successCount*portsPerBatch, pp.PortCount())
}

func TestProvider_AddIndeed(t *testing.T) {
	pp := newProvider("ns_a")

	pp.AddIndeed("owner_a", []int{20000, 20001})
	assert.Equal(t, 0, pp.PortToVerifyCount())
	assert.Equal(t, 2, pp.PortCount())

	ports, err := pp.Allocate(2)
	assert.NilError(t, err)
	assert.Equal(t, 2, pp.PortToVerifyCount())
	assert.Equal(t, 4, pp.PortCount())

	pp.AddIndeed("owner_b", []int{int(ports[0]), int(ports[1])})

	assert.Equal(t, 0, pp.PortToVerifyCount())
	assert.Equal(t, 4, pp.PortCount())

	pp.AddIndeed("owner_c", []int{20001})
	assert.Equal(t, 4, pp.PortCount())
	assert.Equal(t, "owner_a", pp.ports[20001].owner)
}

func TestProvider_DeleteIndeed(t *testing.T) {
	pp := newProvider("ns_a")
	portsA := []int{20000, 20001}
	portsB := []int{20003, 20004}
	portsC := []int{20005, 20006}
	pp.AddIndeed("owner_a", portsA)
	pp.AddIndeed("owner_b", portsB)
	pp.AddIndeed("owner_c", portsC)

	pp.DeleteIndeed("owner_a", portsA)
	assert.Equal(t, 4, pp.PortCount())

	pp.DeleteIndeed("owner_c", []int{20004, 20005, 20006, 20007})
	assert.Equal(t, 2, pp.PortCount())
}

func TestProvider_CheckNotVerified(t *testing.T) {
	pp := newProvider("ns_a")

	portsA, err := pp.Allocate(2)
	assert.NilError(t, err)
	assert.Equal(t, 2, len(portsA))

	portsB, err := pp.Allocate(2)
	assert.NilError(t, err)
	assert.Equal(t, 2, len(portsB))

	assert.Equal(t, 4, pp.PortToVerifyCount())
	assert.Equal(t, 4, pp.PortCount())

	expireTime := time.Now().Unix() - DefaultMaxNotVerifiedPortAge

	pp.portsToVerify[int(portsA[0])].allocateTime = expireTime
	pp.portsToVerify[int(portsA[1])].allocateTime = expireTime

	pp.CheckNotVerified()

	assert.Equal(t, 2, pp.PortToVerifyCount())
	assert.Equal(t, 2, pp.PortCount())
}
