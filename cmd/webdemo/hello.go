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
	"errors"
	"reflect"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/asserts"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

type HelloHandler struct {
}

var _ api.ProtoHandler = (*HelloHandler)(nil)

func (h *HelloHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	req, ok := request.(*HelloRequest)
	if !ok {
		errs.AppendErr(errors.New("unable convert request to HelloRequest"))
		return
	}
	errs.AppendErr(asserts.NotEmpty(req.Name, "request.name is empty"))
}

func (h *HelloHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	req, ok := request.(*HelloRequest)
	if !ok {
		return nil
	}
	if req.Name == "Invaild name" {
		res := &HelloResponse{}
		return res
	}
	c := "hello, " + req.Name
	return &HelloResponse{
		Content: c,
	}
}

func (h *HelloHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(HelloRequest{}), reflect.TypeOf(HelloResponse{})
}
