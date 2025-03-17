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

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyStringProto_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    AnyStringProto
		expected string
	}{
		{
			name:     "normal case",
			input:    AnyStringProto{Content: "Hello Kuscia"},
			expected: "Hello Kuscia",
		},
		{
			name:     "empty case",
			input:    AnyStringProto{Content: ""},
			expected: "",
		},
		{
			name:     "special characters",
			input:    AnyStringProto{Content: `{"key": "value"}`},
			expected: `{"key": "value"}`,
		},
		{
			name:     "unicode characters",
			input:    AnyStringProto{Content: "\\u4f60\\u597d\\uff0cKuscia!"},
			expected: "\\u4f60\\u597d\\uff0cKuscia!",
		},
		{
			name:     "null byte",
			input:    AnyStringProto{Content: "\x00"},
			expected: "\x00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.input.MarshalJSON()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
