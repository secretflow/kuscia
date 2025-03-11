package render

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderToBytes(t *testing.T) {
	tests := []struct {
		name      string
		data      proto.Message
		wantErr   bool
		wantBytes []byte
	}{
		{
			name:      "Valid AnyStringProto",
			data:      &api.AnyStringProto{Content: "Hello World"},
			wantErr:   false,
			wantBytes: []byte("Hello World"),
		},
		{
			name:      "Valid Proto Message",
			data:      &api.AnyStringProto{Content: "Hello World"},
			wantErr:   false,
			wantBytes: []byte("Hello World"),
		},
		{
			name:      "Nil Data",
			data:      nil,
			wantErr:   false,
			wantBytes: []byte("{}"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &JSONRender{Data: tt.data}
			got, err := r.RenderToBytes()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantBytes, got)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		data     proto.Message
		wantErr  bool
		wantBody string
	}{
		{
			name:     "Valid AnyStringProto",
			data:     &api.AnyStringProto{Content: "Hello World"},
			wantErr:  false,
			wantBody: "Hello World",
		},
		{
			name:     "Nil Data",
			data:     nil,
			wantErr:  false,
			wantBody: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			r := &JSONRender{Data: tt.data}
			err := r.Render(recorder)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantBody, recorder.Body.String())
				assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func TestWriteContentType(t *testing.T) {
	tests := []struct {
		name     string
		wantType string
	}{
		{
			name:     "Valid WriteContentType",
			wantType: "application/json; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			r := &JSONRender{}
			r.WriteContentType(recorder)

			assert.Equal(t, tt.wantType, recorder.Header().Get("Content-Type"))
		})
	}
}

type invalidProtoMessage struct{}

func (m *invalidProtoMessage) Reset()         {}
func (m *invalidProtoMessage) String() string { return "invalid" }
func (m *invalidProtoMessage) ProtoMessage()  {}

func (m *invalidProtoMessage) Marshal() ([]byte, error) {
	return nil, fmt.Errorf("mock marshal error")
}
func (m *invalidProtoMessage) ProtoReflect() protoreflect.Message {
	return nil
}
