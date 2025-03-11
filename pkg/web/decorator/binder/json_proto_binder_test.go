package binder

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestJSONProtoBinder_BindBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected error
		obj      interface{}
	}{
		{
			name:     "Valid JSON",
			body:     []byte(`{"key": "value"}`),
			expected: nil,
			obj:      &map[string]string{},
		},
		{
			name:     "Invalid JSON",
			body:     []byte(`{"key": value}`),
			expected: errors.New("invalid character"),
			obj:      &map[string]string{},
		},
		{
			name:     "Valid Protobuf",
			body:     []byte(`{"fields": {"key": {"stringValue": "value"}}}`),
			expected: nil,
			obj:      &structpb.Struct{},
		},
		{
			name:     "Invalid Protobuf",
			body:     []byte(`{"fields": {"key": null}}`),
			expected: nil,
			obj:      &structpb.Struct{},
		},
	}

	b := JSONProtoBinder{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.BindBody(tt.body, tt.obj)
			if tt.expected != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJSONProtoBinder_Bind(t *testing.T) {
	b := JSONProtoBinder{}

	tests := []struct {
		name     string
		body     []byte
		expected error
		obj      interface{}
	}{
		{
			name:     "Valid JSON Request",
			body:     []byte(`{"key": "value"}`),
			expected: nil,
			obj:      &map[string]string{},
		},
		{
			name:     "Empty Request",
			body:     nil,
			expected: errors.New("EOF"),
			obj:      &map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(tt.body))
			err := b.Bind(req, tt.obj)
			if tt.expected != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type mockProtoMessage struct {
	Field1 string `json:"Field1"`
	Field2 int32  `json:"Field2"`
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		obj        interface{}
		wantErr    error
		wantResult *mockProtoMessage
	}{
		{
			name:       "Valid JSON body",
			body:       []byte(`{"Field1": "test", "Field2": 123}`),
			obj:        &mockProtoMessage{},
			wantErr:    nil,
			wantResult: &mockProtoMessage{Field1: "test", Field2: 123},
		},
		{
			name:       "Invalid JSON body (Field2 is a string)",
			body:       []byte(`{"Field1": "test", "Field2": "invalid"}`),
			obj:        &mockProtoMessage{},
			wantErr:    errors.New("json: cannot unmarshal string into Go struct field mockProtoMessage.Field2 of type int32"),
			wantResult: nil,
		},
		{
			name:       "Valid JSON body with unknown fields",
			body:       []byte(`{"Field1": "test", "Field2": 123, "ExtraField": "extra"}`),
			obj:        &mockProtoMessage{},
			wantErr:    errors.New("json: unknown field \"ExtraField\""),
			wantResult: nil,
		},
		{
			name:       "Empty body",
			body:       []byte(``),
			obj:        &mockProtoMessage{},
			wantErr:    errors.New("EOF"),
			wantResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeJSON(bytes.NewReader(tt.body), tt.obj)

			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, tt.obj)
			}
		})
	}
}
