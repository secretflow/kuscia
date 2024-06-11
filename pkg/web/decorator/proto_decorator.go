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
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/decorator/binder"
	bizrender "github.com/secretflow/kuscia/pkg/web/decorator/render"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

const REQUEST = "REQUEST"
const RESPONSE = "RESPONSE"

type BizFlow struct {
	handler    api.ProtoHandler
	ReqType    *reflect.Type
	RespType   *reflect.Type
	BizContext *api.BizContext
}

// ValidateFailedHandler is processor when verification fails.
type ValidateFailedHandler func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse

// UnexpectedErrorHandler is processor when the framework handles exceptions.
type UnexpectedErrorHandler func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse

// PostProcessHandler is post processor after frame processing.
type PostProcessHandler func(response api.ProtoResponse, bizContext *api.BizContext)

// PreRenderHandler is pre processor before rendering results.
type PreRenderHandler func(bizContext *api.BizContext, render bizrender.Render)

type ProtoDecoratorOptions struct {
	// Required
	ValidateFailedHandler
	// Required
	UnexpectedErrorHandler
	// Optional
	PostProcessHandler
	// Optional
	PreRenderHandler
	// Optional, have no use if `MarshalOptions` is set
	RenderJSONUseProtoNames bool
	// Optional
	*protojson.MarshalOptions
}

// ProtoDecorator is the decorator of protocol request processing logic.
// During request processing, ProtoDecorator uses ShouldBindBodyWith of ginContext to read requests with contentType
// of "text/plain", "application/json" and "", so that other middleware can use ShouldBindBodyWith to read the request body again.
func ProtoDecorator(e framework.ConfBeanRegistry, handler api.ProtoHandler, options *ProtoDecoratorOptions) gin.HandlerFunc {
	reqType, respType := handler.GetType()
	if options.MarshalOptions == nil {
		// if `MarshalOptions` not set, set it to default
		options.MarshalOptions = &protojson.MarshalOptions{
			UseProtoNames:   options.RenderJSONUseProtoNames,
			EmitUnpopulated: true,
		}
	}

	return func(context *gin.Context) {
		flow := &BizFlow{
			handler:  handler,
			RespType: &respType,
			ReqType:  &reqType,
			BizContext: &api.BizContext{
				Context:          context,
				Store:            map[string]interface{}{},
				ConfBeanRegistry: e,
				Logger:           logs.GetLogger(),
			},
		}
		bizContext := flow.BizContext
		errs := &errorcode.Errs{}

		request, err := getProtoRequest(flow)
		if err != nil {
			bizContext.Logger.Errorf("get proto request failed, %v", err.Error())
			errs.AppendErr(err)
			renderResponse(bizContext, options.UnexpectedErrorHandler(flow, errs), options.PostProcessHandler, options.PreRenderHandler, *options.MarshalOptions)
			return
		}

		// preCheck
		if doValidate(flow, request, errs); len(*errs) != 0 {
			bizContext.Logger.Error("validate error", fmt.Sprintf("%s", *errs))
			renderResponse(bizContext, options.ValidateFailedHandler(flow, errs), options.PostProcessHandler, options.PreRenderHandler, *options.MarshalOptions)
			return
		}

		// doBizProcess
		if resp := doBizProcess(flow, request, errs); len(*errs) != 0 {
			renderResponse(bizContext, options.UnexpectedErrorHandler(flow, errs), options.PostProcessHandler, options.PreRenderHandler, *options.MarshalOptions)
		} else {
			renderResponse(bizContext, resp, options.PostProcessHandler, options.PreRenderHandler, *options.MarshalOptions)
		}
	}
}

// getProtoRequest gets proto structured request.
func getProtoRequest(flow *BizFlow) (api.ProtoRequest, error) {
	bizContext := flow.BizContext
	if *flow.ReqType == reflect.TypeOf(api.AnyStringProto{}) {
		c, err := bizContext.Context.GetRawData()
		if err != nil {
			return nil, err
		}
		return &api.AnyStringProto{Content: string(c)}, nil
	}

	// Create pointer of ProtoRequest corresponding value object through reflection.
	request, ok := reflect.New(*flow.ReqType).Interface().(api.ProtoRequest)
	if !ok {
		return nil, errors.New("invalidate proto type")
	}

	// Get request should define struct with gin form tag.
	if bizContext.Request.Method == http.MethodGet {
		bizContext.Logger.Info(REQUEST, bizContext.Request.RequestURI)
		if err := bizContext.Context.ShouldBindQuery(request); err != nil {
			return nil, err
		}
		return request, nil
	}

	// Automatic type conversion.
	contentType := bizContext.Context.GetHeader("Content-Type")
	if contentType == "" || contentType == "text/plain" || strings.HasPrefix(contentType, "application/json") {
		// Use ShouldBindBodyWith allowed other middleware read context again
		if err := bizContext.Context.ShouldBindBodyWith(request, binder.JSONProtoBinder{}); err != nil {
			return nil, err
		}
	} else if err := bizContext.Context.ShouldBind(request); err != nil {
		return nil, err
	}

	bizContext.Logger.Info(REQUEST, fmt.Sprintf("  Path: %s  Content-Type: %s  Body: %s", bizContext.Request.RequestURI, contentType, messageToJSONString(request)))
	return request, nil
}

func messageToJSONString(m proto.Message) string {
	if m != nil && m.ProtoReflect() != nil {
		bytes, err := protojson.Marshal(m)
		if err != nil {
			return fmt.Sprintf("%s (protojson marshal err: %s)", m, err)
		}
		return string(bytes)
	}

	return fmt.Sprintf("%v", m)
}

func doValidate(flow *BizFlow, request api.ProtoRequest, errs *errorcode.Errs) {
	defer func() {
		if e := recover(); e != nil {
			if e != errs {
				errs.AppendErr(errors.New(fmt.Sprintf("%s", e)))
				flow.BizContext.Logger.Error("doValidate panic", fmt.Sprintf("type=%s, %v", reflect.TypeOf(e), e))
				debug.PrintStack()
			}
		}
	}()

	flow.handler.Validate(flow.BizContext, request, errs)
}

// doBizProcess processes business logic.
func doBizProcess(flow *BizFlow, request api.ProtoRequest, errs *errorcode.Errs) (resp api.ProtoRequest) {
	// In case of panic() is called in user code.
	defer func() {
		if e := recover(); e != nil {
			errs.AppendErr(fmt.Errorf("doBizProcess panic, err: %v", e))
			flow.BizContext.Logger.Error("doBizProcess panic", fmt.Sprintf("%v", e))

			debug.PrintStack()
		}
	}()
	// Request processing logic.
	resp = flow.handler.Handle(flow.BizContext, request)
	return resp
}

// renderResponse renders response results.
func renderResponse(ctx *api.BizContext, response api.ProtoResponse, postProcessHandler PostProcessHandler, preRenderHandler PreRenderHandler, jsonMarshalOptions protojson.MarshalOptions) {
	if postProcessHandler != nil {
		postProcessHandler(response, ctx)
	}
	ctx.Logger.Info(RESPONSE, messageToJSONString(response))

	var render bizrender.Render
	switch ctx.Context.ContentType() {
	case binding.MIMEPROTOBUF:
		render = &bizrender.ProtoRender{Data: response}
	default:
		render = &bizrender.JSONRender{Data: response, MarshalOptions: jsonMarshalOptions}
	}

	if preRenderHandler != nil {
		preRenderHandler(ctx, render)
	}

	ctx.Context.Render(http.StatusOK, render)
}

// DefaultProtoDecoratorMaker returns default ProtoDecorator.
func DefaultProtoDecoratorMaker(validateFailedCode int32, unexpectedECode int32) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   setDefaultErrorResp(validateFailedCode),
			UnexpectedErrorHandler:  setDefaultErrorResp(unexpectedECode),
			RenderJSONUseProtoNames: true,
		})
	}
}

// InterConnProtoDecoratorMaker returns inter connection ProtoDecorator.
func InterConnProtoDecoratorMaker(validateFailedCode int32, unexpectedECode int32) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   setInterConnErrorResp(validateFailedCode),
			UnexpectedErrorHandler:  setInterConnErrorResp(unexpectedECode),
			RenderJSONUseProtoNames: true,
		})
	}
}

// CustomProtoDecoratorMaker returns custom connection ProtoDecorator.
func CustomProtoDecoratorMaker(validateFailedHandler ValidateFailedHandler, unexpectedErrorHandler UnexpectedErrorHandler) func(framework.ConfBeanRegistry, api.ProtoHandler) gin.HandlerFunc {
	return func(e framework.ConfBeanRegistry, handler api.ProtoHandler) gin.HandlerFunc {
		return ProtoDecorator(e, handler, &ProtoDecoratorOptions{
			ValidateFailedHandler:   validateFailedHandler,
			UnexpectedErrorHandler:  unexpectedErrorHandler,
			RenderJSONUseProtoNames: true,
		})
	}
}

// setDefaultErrorResp sets default format error response with the error code.
func setDefaultErrorResp(errCode int32) func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	return func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
		wrappedErr := fmt.Errorf("%s", errs)
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

// setInterConnErrorResp sets interconn format error response with the error code.
func setInterConnErrorResp(errCode int32) func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
	return func(flow *BizFlow, errs *errorcode.Errs) (response api.ProtoResponse) {
		wrappedErr := fmt.Errorf("%s", errs)
		resp := map[string]interface{}{
			"code":    errCode,
			"message": wrappedErr.Error(),
		}
		bytes, _ := json.Marshal(resp)
		response = &api.AnyStringProto{
			Content: string(bytes),
		}
		return response
	}
}
