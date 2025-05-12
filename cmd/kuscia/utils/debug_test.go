// Copyright 2025 Ant Group Co., Ltd.
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

package utils

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetupPprof_DebugEnabled(t *testing.T) {
	debugPort := 29090
	SetupPprof(true, debugPort)

	// Allow some time for the server to start
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:29090/debug/pprof/")
	assert.NoError(t, err, "HTTP request to pprof endpoint should succeed")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "pprof endpoint should return status OK")
}

func TestSetupPprof_DebugDisabled(t *testing.T) {
	// Debug mode disabled, no server should start
	SetupPprof(false, 0)

	// Allow some time to ensure no server starts
	time.Sleep(100 * time.Millisecond)

	_, err := http.Get("http://localhost:28080/debug/pprof/")
	assert.Error(t, err, "HTTP request to pprof endpoint should fail when debug is disabled")
}