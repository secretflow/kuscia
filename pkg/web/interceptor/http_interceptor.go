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

package interceptor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
)

type responseWithBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write Override the original method to capture response body data
func (w *responseWithBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString Override the original method to capture response body data
func (w *responseWithBodyWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// ResponseBody Return the captured response body
func (w *responseWithBodyWriter) ResponseBody() string {
	return w.body.String()
}

func newResponseWithBodyWriter(rw gin.ResponseWriter) *responseWithBodyWriter {
	return &responseWithBodyWriter{
		ResponseWriter: rw,
		body:           &bytes.Buffer{},
	}
}

func HTTPServerLoggingInterceptor(logger nlog.NLog) func(c *gin.Context) {
	return func(c *gin.Context) {
		var r = c.Request
		withBodyWriter := newResponseWithBodyWriter(c.Writer)
		c.Writer = withBodyWriter
		// log prefix : request method and URI
		prefix := r.Method + " " + r.URL.RequestURI()
		// request Body
		var requestBody = make([]byte, 0)
		if r.Body != nil && r.ContentLength < maxSizeBytes {
			var err error
			if requestBody, err = io.ReadAll(r.Body); err != nil {
				_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("[%s] read request body failed: %v", prefix, err))
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		// request start time
		startTime := time.Now()
		defer safeLog(logger, protocolHTTP, func() {
			// request processing time
			duration := time.Since(startTime)
			var errors []error
			emptyBody := make([]byte, 0)

			if c.Err() != nil {
				errors = append(errors, c.Err())
			}

			context := &loggerContext{
				Protocol:      protocolHTTP,
				RequestMethod: r.Method,
				RequestPath:   r.URL.RequestURI(),
				XForwardHost:  r.Header.Get(constants.XForwardHostHeader),
				ContextType:   c.Writer.Header().Get(constants.ContentTypeHeader),
				StatusCode:    c.Writer.Status(),
				Duration:      duration,
				Errs:          errors,
				RequestBody:   emptyBody,
				ResponseBody:  emptyBody,
			}

			if !hasSensitiveHTTPPathPrefix(context.RequestPath) {
				errors = adjustHTTPRequestAndResponse(logger, context, requestBody, withBodyWriter)
			}
			printfLoggerContext(logger, context)
		})
		// run next handler
		c.Next()
	}
}

func adjustHTTPRequestAndResponse(logger nlog.NLog, ctx *loggerContext, reqBody []byte, respBodyWriter *responseWithBodyWriter) (errors []error) {
	var structInterface any
	// request body
	if len(reqBody) > 0 {
		if unmarshalReqErr := json.Unmarshal(reqBody, &structInterface); unmarshalReqErr != nil {
			errors = append(errors, unmarshalReqErr)
		} else {
			requestMap := utils.StructToMap(structInterface, sensitiveFields...)
			if requestJSON, marshalReqErr := json.Marshal(requestMap); marshalReqErr != nil {
				errors = append(errors, marshalReqErr)
			} else {
				ctx.RequestBody = requestJSON
			}
		}
	}
	// response body
	responseBodyLen := respBodyWriter.body.Len()
	logger.Debugf("response body len: %d", respBodyWriter.body.Len())
	if responseBodyLen < maxSizeBytes {
		if unmarshalRespErr := json.Unmarshal([]byte(respBodyWriter.ResponseBody()), &structInterface); unmarshalRespErr != nil {
			errors = append(errors, unmarshalRespErr)
		} else {
			responseMap := utils.StructToMap(structInterface, sensitiveFields...)
			if responseJSON, marshalRespErr := json.Marshal(responseMap); marshalRespErr != nil {
				errors = append(errors, marshalRespErr)
			} else {
				ctx.ResponseBody = responseJSON
			}
		}
	}
	return errors
}

func HTTPTokenAuthInterceptor(tokenData string) func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader(constants.TokenHeader)
		err := tokenCheck([]string{token}, tokenData)
		if err != nil {
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		c.Next()
	}
}

func HTTPSetMasterRoleInterceptor() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set(constants.AuthRole, constants.AuthRoleMaster)
		c.Set(constants.SourceDomainKey, constants.AuthRoleMaster)
		c.Next()
	}
}

func HTTPSourceAuthInterceptor() func(c *gin.Context) {
	return func(c *gin.Context) {
		source := c.GetHeader(constants.SourceDomainHeader)
		if source == "" {
			err := status.Errorf(codes.Unauthenticated, "source domain header not found")
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		c.Set(constants.AuthRole, constants.AuthRoleDomain)
		c.Set(constants.SourceDomainKey, source)
		c.Next()
	}
}
