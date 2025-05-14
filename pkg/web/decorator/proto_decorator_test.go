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

package decorator

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

type testHandler struct{}

func (h *testHandler) Validate(ctx *api.BizContext, req api.ProtoRequest, errs *errorcode.Errs) {
}

func (h *testHandler) Handle(ctx *api.BizContext, req api.ProtoRequest) api.ProtoResponse {
	return &api.AnyStringProto{Content: "test_response"}
}

func (h *testHandler) GetType() (req, resp reflect.Type) {
	return reflect.TypeOf(&api.AnyStringProto{}), reflect.TypeOf(&api.AnyStringProto{})
}

func TestProtoDecorator(t *testing.T) {
	engine := gin.New()
	handler := &testHandler{}

	options := &ProtoDecoratorOptions{
		ValidateFailedHandler: func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse {
			return &api.AnyStringProto{Content: "validation_failed"}
		},
		UnexpectedErrorHandler: func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse {
			return &api.AnyStringProto{Content: "unexpected_error"}
		},
		MarshalOptions: &protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: true,
		},
	}

	engine.POST("/test", ProtoDecorator(nil, handler, options))

	t.Run("Invalid content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString("<xml></xml>"))
		req.Header.Set("Content-Type", "text/xml")

		engine.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), "unexpected_error")
	})
}

func TestGetProtoRequest(t *testing.T) {

	t.Run("Handle GET requests", func(t *testing.T) {
		reqType := reflect.TypeOf(api.AnyStringProto{})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?content=test", nil)

		flow := &BizFlow{
			ReqType:    &reqType,
			BizContext: &api.BizContext{Context: c},
		}

		req, err := getProtoRequest(flow)
		assert.NoError(t, err)
		assert.IsType(t, &api.AnyStringProto{}, req)
	})
}

func TestRenderResponse(t *testing.T) {
	t.Run("JSON rendering", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Accept", "application/json")
		ctx := &api.BizContext{Context: c}

		opts := protojson.MarshalOptions{UseProtoNames: true}
		renderResponse(ctx, &api.AnyStringProto{Content: "test"}, nil, nil, opts)

		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	})
}
