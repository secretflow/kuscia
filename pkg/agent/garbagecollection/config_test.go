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

package garbagecollection

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// test default gc config
func TestConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultGCConfig()
	assert.Equal(t, cfg.GCInterval, 10*time.Minute)
	assert.Equal(t, cfg.MaxDeleteNum, 100)
}

func TestAttribute(t *testing.T) {
	t.Parallel()
	cfg := DefaultLogFileGCConfig()
	assert.Equal(t, cfg.PodNamePattern, ".*_(.*)_.*")
	assert.Equal(t, cfg.IsDurationCheck, true)
	assert.Equal(t, cfg.IsRecursiveDurationCheck, true)
	assert.Equal(t, cfg.GCDuration, 30*24*time.Hour)
}
