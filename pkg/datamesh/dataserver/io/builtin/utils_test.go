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

package builtin

import (
	"testing"

	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/memory"
)

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  float64
	}{
		{
			name:      "Valid float64",
			input:     "123.456",
			expectErr: false,
			expected:  123.456,
		},
		{
			name:      "Invalid float64",
			input:     "invalid",
			expectErr: true,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "Valid negative float64",
			input:     "-987.654",
			expectErr: false,
			expected:  -987.654,
		},
		{
			name:      "Valid float64 with exponent",
			input:     "1.23e4",
			expectErr: false,
			expected:  12300.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewFloat64Builder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseFloat64(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewFloat64Array()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}
func TestParseBool(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  bool
	}{
		{"Valid true", "true", false, true},
		{"Valid false", "false", false, false},
		{"Invalid bool", "invalid", true, false},
		{"Empty string", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewBooleanBuilder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseBool(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewBooleanArray()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}

func TestParseInt8(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int8
	}{
		{"Valid int8", "127", false, 127},
		{"Invalid int8", "invalid", true, 0},
		{"Out of range", "128", true, 0},
		{"Empty string", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewInt8Builder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseInt8(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewInt8Array()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}

func TestParseInt16(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int16
	}{
		{"Valid int16", "32767", false, 32767},
		{"Invalid int16", "invalid", true, 0},
		{"Out of range", "32768", true, 0},
		{"Empty string", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewInt16Builder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseInt16(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewInt16Array()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}

func TestParseInt32(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int32
	}{
		{"Valid int32", "2147483647", false, 2147483647},
		{"Invalid int32", "invalid", true, 0},
		{"Out of range", "2147483648", true, 0},
		{"Empty string", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewInt32Builder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseInt32(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewInt32Array()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int64
	}{
		{"Valid int64", "9223372036854775807", false, 9223372036854775807},
		{"Invalid int64", "invalid", true, 0},
		{"Out of range", "9223372036854775808", true, 0},
		{"Empty string", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewInt64Builder(memory.NewGoAllocator())
			defer builder.Release()

			err := ParseInt64(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
				}
				arr := builder.NewInt64Array()
				defer arr.Release()
				if arr.Value(0) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, arr.Value(0))
				}
			}
		})
	}
}
