//go:build !linux
// +build !linux

// Copyright 2024 Ant Group Co., Ltd.
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

package cgroup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	cpuQuota    = int64(100000)
	cpuPeriod   = uint64(100000)
	memoryLimit = int64(100000)
)

func newMockManager() (Manager, error) {
	return NewManager(nil)
}

func TestAddCgroup(t *testing.T) {
	m, _ := newMockManager()
	got := m.AddCgroup()
	assert.NotNil(t, got)
}

func TestUpdateCgroup(t *testing.T) {
	m, _ := newMockManager()
	got := m.UpdateCgroup()
	assert.NotNil(t, got)
}

func TestDeleteCgroup(t *testing.T) {
	m, _ := newMockManager()
	got := m.DeleteCgroup()
	assert.NotNil(t, got)
}

func TestHasPermission(t *testing.T) {
	got := HasPermission()
	assert.False(t, got)
}

func TestIsCgroupExist(t *testing.T) {
	got := IsCgroupExist("test", false)
	assert.False(t, got)
}

func TestGetMemoryLimit(t *testing.T) {
	_, got := GetMemoryLimit("test")
	assert.NotNil(t, got)
}

func TestGetMemoryUsage(t *testing.T) {
	_, got := GetMemoryUsage("test")
	assert.NotNil(t, got)
}
