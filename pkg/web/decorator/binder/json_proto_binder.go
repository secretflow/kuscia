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

package binder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type JSONProtoBinder struct{}

func (JSONProtoBinder) Name() string {
	return "json"
}

func (b JSONProtoBinder) Bind(req *http.Request, obj interface{}) error {
	if req == nil || req.Body == nil {
		return fmt.Errorf("invalid request")
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return b.BindBody(body, obj)
}

func (JSONProtoBinder) BindBody(body []byte, obj interface{}) error {
	if m, ok := obj.(protoreflect.ProtoMessage); ok {
		err := protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, m)
		// In order to be compatible with the following situations:
		// {"annotations":{"name":null}}
		// protojson cannot resolve null in the map. If protojson fails to parse, it will be parsed again with normal json.
		if err == nil {
			return err
		}
	}
	return decodeJSON(bytes.NewReader(body), obj)
}

func decodeJSON(r io.Reader, obj interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return nil
}
