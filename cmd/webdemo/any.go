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

package main

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

type AnyHandler struct {
}

var _ api.ProtoHandler = (*AnyHandler)(nil)

func (h *AnyHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	// Unsupported
}

func (h *AnyHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	return &api.AnyStringProto{
		Content: `{"header": {"is_success": true}, "content": "hahaha"}`,
	}
}

func (h *AnyHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(api.AnyStringProto{}), reflect.TypeOf(api.AnyStringProto{})
}
