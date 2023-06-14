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

package decorator

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

// ProtoDecoratorMaker builds ProtoDecorator for filling common header.
func ProtoDecoratorMaker(validateFailedCode int32, unexpectedECode int32) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   SetErrorRespHeader(validateFailedCode),
			UnexpectedErrorHandler:  SetErrorRespHeader(unexpectedECode),
			PostProcessHandler:      SetTraceIDToRespHeader,
			RenderJSONUseProtoNames: true,
		})
	}
}

// ProtoDecoratorWithoutProtoNamesRenderMaker builds ProtoDecorator for filling common header. Do not output by tag when serializing.
func ProtoDecoratorWithoutProtoNamesRenderMaker(validateFailedCode int32, unexpectedECode int32) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   SetErrorRespHeader(validateFailedCode),
			UnexpectedErrorHandler:  SetErrorRespHeader(unexpectedECode),
			PostProcessHandler:      SetTraceIDToRespHeader,
			RenderJSONUseProtoNames: false,
		})
	}
}

// AnyStringDecoratorMaker is used to wrap request handler.
func AnyStringDecoratorMaker(validateFailedCode int32, unexpectedECode int32) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   SetErrorResp(validateFailedCode),
			UnexpectedErrorHandler:  SetErrorResp(unexpectedECode),
			RenderJSONUseProtoNames: true,
		})
	}
}

// SetAnyStringErrorRespHeader sets common error response header with the error message.
func SetAnyStringErrorRespHeader(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	wrappedErr := errors.Errorf("%s", fmt.Errorf("%s", *errs))
	resp := map[string]map[string]interface{}{
		"header": {
			"is_success": false,
			"trace_info": flow.BizContext.TraceID,
			"error_msg":  wrappedErr.Error(),
		},
	}
	bytes, _ := json.Marshal(resp)
	response = &api.AnyStringProto{
		Content: string(bytes),
	}
	return
}

// SetErrorResp is used to set error response.
func SetErrorResp(errCode int32) func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	return func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
		wrappedErr := errors.Errorf("%s", fmt.Errorf("%s", *errs))
		resp := map[string]map[string]interface{}{
			"status": {
				"code":    errCode,
				"message": wrappedErr.Error(),
			},
		}
		bytes, _ := json.Marshal(resp)
		response = &api.AnyStringProto{
			Content: string(bytes),
		}
		return response
	}
}

// SetErrorRespHeader sets common error response header with the error code.
func SetErrorRespHeader(errCode int32) func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	return func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
		errMsg := fmt.Sprintf("Errors: '%s'", *errs)
		respType := flow.RespType
		respValue := reflect.New(*respType)
		res, ok := respValue.Interface().(api.ProtoResponse)
		if !ok {
			flow.BizContext.Logger.Warn("Unable convert respValue to ProtoResponse")
			response = SetAnyStringErrorRespHeader(flow, errs)
			return
		}
		if err := SetCommonHeader(res, false, errCode, errMsg); err != nil {
			flow.BizContext.Logger.Warn("SetErrorRespHeader panic", fmt.Sprintf("%s", err))
			response = SetAnyStringErrorRespHeader(flow, errs)
			return
		}
		SetTraceIDToRespHeader(res, flow.BizContext)
		return res
	}
}

// SetCommonHeader sets the common header for the response, which can be called directly in the handler.
// To add trace_id, also call SetTraceIDToRespHeader.
func SetCommonHeader(response api.ProtoResponse, succ bool, code int32, msg string) (err error) {
	defer func() {
		// Println executes normally even if there is a panic
		if recerr := recover(); err != nil {
			err = fmt.Errorf("SetCommonHeader panic %s", recerr)
		}
	}()
	respValue := reflect.ValueOf(response)
	header := respValue.Elem().FieldByName("Header")
	if header.IsNil() {
		headerValue := reflect.New(header.Type().Elem())
		header.Set(headerValue)
	}
	if err := setValue(header.Elem().FieldByName("IsSuccess"), reflect.ValueOf(succ)); err != nil {
		return err
	}
	if code != 0 {
		if err := setValue(header.Elem().FieldByName("ErrorCode"), reflect.ValueOf(int32(code))); err != nil {
			return err
		}
	}
	if len(msg) != 0 {
		if err := setValue(header.Elem().FieldByName("ErrorMsg"), reflect.ValueOf(msg)); err != nil {
			return err
		}
	}
	return nil
}

// SetTraceIDToRespHeader sets trace id to the response header.
func SetTraceIDToRespHeader(response api.ProtoResponse, bizContext *api.BizContext) {
	defer func() {
		// Println executes normally even if there is a panic
		if err := recover(); err != nil {
			bizContext.Logger.Warn("SetTraceIDToRespHeader panic", fmt.Sprintf("%s", err))
		}
	}()
	header := reflect.ValueOf(response).Elem().FieldByName("Header")
	if header.IsNil() {
		value := reflect.New(header.Type().Elem())
		_ = setValue(value.Elem().FieldByName("IsSuccess"), reflect.ValueOf(true))
		trace := value.Elem().FieldByName("TraceInfo")
		_ = setValue(trace, reflect.ValueOf(bizContext.TraceID))
		header.Set(value)
	} else {
		trace := header.Elem().FieldByName("TraceInfo")
		if trace.IsZero() {
			_ = setValue(trace, reflect.ValueOf(bizContext.TraceID))
		}
	}
}

// setValue For V1/V2 common header
func setValue(elem reflect.Value, val reflect.Value) error {
	if elem.Kind() == reflect.Ptr {
		if elem.IsNil() {
			elem.Set(reflect.New(elem.Type().Elem()))
		}
		elem = elem.Elem()
	}
	if elem.Type() == val.Type() {
		elem.Set(val)
		return nil
	}
	return fmt.Errorf("setvalue type mismatch, get(%s), want(%s)", elem.Type().Name(), val.Type().Name())
}
