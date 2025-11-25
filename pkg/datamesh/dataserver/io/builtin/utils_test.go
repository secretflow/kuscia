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

	"github.com/apache/arrow/go/v13/arrow"
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

func TestParseStr2Date32(t *testing.T) {
	mem := memory.NewGoAllocator()
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int32
		isNull    bool
	}{
		{
			name:      "Valid date format",
			input:     "2023-12-25",
			expectErr: false,
			expected:  19716, // 2023-12-25 converted to days since epoch (1970-01-01)
			isNull:    false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "NULL string",
			input:     "NULL",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "Invalid date format",
			input:     "2023-25-12",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Malformed date",
			input:     "invalid-date",
			expectErr: true,
			isNull:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewDate32Builder(mem)
			defer builder.Release()

			err := ParseStr2Date32(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.isNull {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewDate32Array()
				defer arr.Release()
				if !arr.IsNull(0) {
					t.Errorf("expected null value, got valid value: %v", arr.Value(0))
				}
			} else {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewDate32Array()
				defer arr.Release()
				if arr.IsNull(0) {
					t.Errorf("expected valid value, got null")
				} else if int32(arr.Value(0)) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int32(arr.Value(0)))
				}
			}
		})
	}
}

func TestParseStr2Date64(t *testing.T) {
	mem := memory.NewGoAllocator()
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int64
		isNull    bool
	}{
		{
			name:      "Valid date format",
			input:     "2023-12-25",
			expectErr: false,
			expected:  1703462400000, // 2023-12-25 00:00:00 UTC in milliseconds
			isNull:    false,
		},
		{
			name:      "Valid datetime format",
			input:     "2023-12-25 14:30:45",
			expectErr: false,
			expected:  1703514645000, // 2023-12-25 14:30:45 ms
			isNull:    false,
		},
		{
			name:      "Valid datetime with milliseconds",
			input:     "2023-12-25 14:30:45.123",
			expectErr: false,
			expected:  1703514645123,
			isNull:    false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "NULL string",
			input:     "NULL",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "Invalid date format",
			input:     "2023-25-12 25:70:90",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Malformed datetime",
			input:     "invalid-datetime",
			expectErr: true,
			isNull:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewDate64Builder(mem)
			defer builder.Release()

			err := ParseStr2Date64(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.isNull {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewDate64Array()
				defer arr.Release()
				if !arr.IsNull(0) {
					t.Errorf("expected null value, got valid value: %v", arr.Value(0))
				}
			} else {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewDate64Array()
				defer arr.Release()
				if arr.IsNull(0) {
					t.Errorf("expected valid value, got null")
				} else if int64(arr.Value(0)) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int64(arr.Value(0)))
				}
			}
		})
	}
}

func TestParseStr2Time32(t *testing.T) {
	mem := memory.NewGoAllocator()
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int32
		isNull    bool
	}{
		{
			name:      "Valid time format",
			input:     "14:30:45",
			expectErr: false,
			expected:  52245, // 14*3600 + 30*60 + 45 = 52245 seconds
			isNull:    false,
		},
		{
			name:      "Midnight",
			input:     "00:00:00",
			expectErr: false,
			expected:  0,
			isNull:    false,
		},
		{
			name:      "End of day",
			input:     "23:59:59",
			expectErr: false,
			expected:  86399, // 23*3600 + 59*60 + 59 = 86399 seconds
			isNull:    false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "NULL string",
			input:     "NULL",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "Invalid time format",
			input:     "25:70:90",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Malformed time",
			input:     "invalid-time",
			expectErr: true,
			isNull:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewTime32Builder(mem, arrow.FixedWidthTypes.Time32s.(*arrow.Time32Type))
			defer builder.Release()

			err := ParseStr2Time32(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.isNull {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTime32Array()
				defer arr.Release()
				if !arr.IsNull(0) {
					t.Errorf("expected null value, got valid value: %v", arr.Value(0))
				}
			} else {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTime32Array()
				defer arr.Release()
				if arr.IsNull(0) {
					t.Errorf("expected valid value, got null")
				} else if int32(arr.Value(0)) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int32(arr.Value(0)))
				}
			}
		})
	}
}

func TestParseStr2Time64(t *testing.T) {
	mem := memory.NewGoAllocator()
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int64
		isNull    bool
	}{
		{
			name:      "Valid time with microseconds",
			input:     "14:30:45.123456",
			expectErr: false,
			expected:  52245123456, // (14*3600 + 30*60 + 45)*1e6 + 123456 = 52245123456 microseconds
			isNull:    false,
		},
		{
			name:      "Valid time without microseconds",
			input:     "14:30:45",
			expectErr: false,
			expected:  52245000000, // (14*3600 + 30*60 + 45)*1e6 = 52245000000 microseconds
			isNull:    false,
		},
		{
			name:      "Midnight with microseconds",
			input:     "00:00:00.999999",
			expectErr: false,
			expected:  999999,
			isNull:    false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "NULL string",
			input:     "NULL",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "Invalid time format",
			input:     "25:70:90.123456",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Malformed time",
			input:     "invalid-time",
			expectErr: true,
			isNull:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewTime64Builder(mem, arrow.FixedWidthTypes.Time64us.(*arrow.Time64Type))
			defer builder.Release()

			err := ParseStr2Time64(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.isNull {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTime64Array()
				defer arr.Release()
				if !arr.IsNull(0) {
					t.Errorf("expected null value, got valid value: %v", arr.Value(0))
				}
			} else {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTime64Array()
				defer arr.Release()
				if arr.IsNull(0) {
					t.Errorf("expected valid value, got null")
				} else if int64(arr.Value(0)) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int64(arr.Value(0)))
				}
			}
		})
	}
}

func TestParseStr2Timestamp(t *testing.T) {
	mem := memory.NewGoAllocator()
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  int64
		isNull    bool
	}{
		{
			name:      "Valid date format",
			input:     "2023-12-25",
			expectErr: false,
			expected:  1703462400, // 2023-12-25 00:00:00 UTC
			isNull:    false,
		},
		{
			name:      "Valid datetime format",
			input:     "2023-12-25 14:30:45",
			expectErr: false,
			expected:  1703514645, // 2023-12-25 14:30:45
			isNull:    false,
		},
		{
			name:      "Valid datetime with milliseconds",
			input:     "2023-12-25 14:30:45.123",
			expectErr: false,
			expected:  1703514645, // 2023-12-25 14:30:45
			isNull:    false,
		},
		{
			name:      "Valid ISO datetime",
			input:     "2023-12-25T14:30:45Z",
			expectErr: false,
			expected:  1703514645,
			isNull:    false,
		},
		{
			name:      "Valid ISO datetime with milliseconds",
			input:     "2023-12-25T14:30:45.999Z",
			expectErr: false,
			expected:  1703514645,
			isNull:    false,
		},
		{
			name:      "Unix timestamp as string",
			input:     "1703514645",
			expectErr: false,
			expected:  1703514645,
			isNull:    false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "NULL string",
			input:     "NULL",
			expectErr: false,
			isNull:    true,
		},
		{
			name:      "Invalid datetime format",
			input:     "2023-25-12 25:70:90",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Invalid unix timestamp",
			input:     "invalid-timestamp",
			expectErr: true,
			isNull:    true,
		},
		{
			name:      "Negative unix timestamp",
			input:     "-86400",
			expectErr: false,
			expected:  -86400,
			isNull:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := array.NewTimestampBuilder(mem, arrow.FixedWidthTypes.Timestamp_s.(*arrow.TimestampType))
			defer builder.Release()

			err := ParseStr2Timestamp(builder, tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.isNull {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTimestampArray()
				defer arr.Release()
				if !arr.IsNull(0) {
					t.Errorf("expected null value, got valid value: %v", arr.Value(0))
				}
			} else {
				if builder.Len() != 1 {
					t.Errorf("expected builder to have 1 value, but got %d", builder.Len())
					return
				}
				arr := builder.NewTimestampArray()
				defer arr.Release()
				if arr.IsNull(0) {
					t.Errorf("expected valid value, got null")
				} else if int64(arr.Value(0)) != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, int64(arr.Value(0)))
				}
			}
		})
	}
}
