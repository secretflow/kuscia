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

package rest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetServerAPIPrefix(t *testing.T) {
	c := getDefaultConfig()

	testCases := []struct {
		name     string
		prefix   string
		expected string
	}{
		{
			name:     "set prefix to kuscia-storage",
			prefix:   "kuscia-storage",
			expected: "kuscia-storage",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := SetServerAPIPrefix(tc.prefix)
			opt(c)
			assert.Equal(t, tc.expected, c.serverAPIPrefix)
		})
	}
}

func TestSetRetryTimes(t *testing.T) {
	c := getDefaultConfig()

	testCases := []struct {
		name     string
		times    int
		expected int
	}{
		{
			name:     "set retry times to 10",
			times:    10,
			expected: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := SetRetryTimes(tc.times)
			opt(c)
			assert.Equal(t, tc.expected, c.retryTimes)
		})
	}
}

func TestSetConnectTimeout(t *testing.T) {
	c := getDefaultConfig()

	testCases := []struct {
		name     string
		timeout  int
		expected time.Duration
	}{
		{
			name:     "set timeout to 10",
			timeout:  10,
			expected: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := SetConnectTimeout(tc.timeout)
			opt(c)
			assert.Equal(t, tc.expected, c.connectTimeout)
		})
	}
}

func TestSetReadWriteTimeout(t *testing.T) {
	c := getDefaultConfig()

	testCases := []struct {
		name     string
		timeout  int
		expected time.Duration
	}{
		{
			name:     "set timeout to 10",
			timeout:  10,
			expected: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := SetReadWriteTimeout(tc.timeout)
			opt(c)
			assert.Equal(t, tc.expected, c.readWriteTimeout)
		})
	}
}

func TestSetKeepAliveTimeout(t *testing.T) {
	c := getDefaultConfig()

	testCases := []struct {
		name     string
		timeout  int
		expected time.Duration
	}{
		{
			name:     "set timeout to 10",
			timeout:  10,
			expected: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt := SetKeepAliveTimeout(tc.timeout)
			opt(c)
			assert.Equal(t, tc.expected, c.keepAliveTimeout)
		})
	}
}
