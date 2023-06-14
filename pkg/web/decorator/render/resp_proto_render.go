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

package render

import (
	"net/http"

	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/web/api"
)

// ProtoRender contains the given interface object.
type ProtoRender struct {
	Data  proto.Message
	bytes []byte
	err   error
}

var protoContentType = []string{"application/x-protobuf"}

// RenderToBytes render data to bytes.
func (r *ProtoRender) RenderToBytes() ([]byte, error) {
	if r.bytes != nil || r.err != nil {
		return r.bytes, r.err
	}
	if anyStringProtoData, ok := r.Data.(*api.AnyStringProto); ok {
		r.bytes = []byte(anyStringProtoData.Content)
	} else {
		r.bytes, r.err = proto.Marshal(r.Data)
	}
	return r.bytes, r.err
}

// Render (ProtoBuf) marshals the given interface object and writes data with custom ContentType.
func (r *ProtoRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	bytes, err := r.RenderToBytes()
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}

// WriteContentType (ProtoBuf) writes ProtoBuf ContentType.
func (r *ProtoRender) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, protoContentType)
}
