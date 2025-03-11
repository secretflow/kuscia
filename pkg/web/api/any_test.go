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
			input:    AnyStringProto{Content: "你好，Kuscia!"},
			expected: "你好，Kuscia!",
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
