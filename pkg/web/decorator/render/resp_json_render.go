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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/web/api"
)

type JSONRender struct {
	Data proto.Message
	protojson.MarshalOptions
	bytes []byte
	err   error
}

var jsonContentType = []string{"application/json; charset=utf-8"}

// RenderToBytes renders data to bytes.
func (r *JSONRender) RenderToBytes() ([]byte, error) {
	if r.bytes != nil || r.err != nil {
		return r.bytes, r.err
	}
	if anyStringProtoData, ok := r.Data.(*api.AnyStringProto); ok {
		r.bytes = []byte(anyStringProtoData.Content)
	} else {
		r.bytes, r.err = r.MarshalOptions.Marshal(r.Data)
	}
	return r.bytes, r.err
}

// Render writes data with custom ContentType.
func (r *JSONRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	bytes, err := r.RenderToBytes()
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}

// WriteContentType writes custom ContentType.
func (r *JSONRender) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, jsonContentType)
}

func writeContentType(w http.ResponseWriter, value []string) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = value
	}
}
