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

package nlog

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type Formatter interface {
	Format(context.Context, string) string
}

type defaultFormatter struct {
}

func (f *defaultFormatter) Format(ctx context.Context, log string) string {
	return log
}

func NewDefaultFormatter() Formatter {
	return &defaultFormatter{}
}

type ginLogFormatter struct {
}

// new log 的日志格式
func (f *ginLogFormatter) Format(ctx context.Context, log string) string {
	if ctx == nil {
		return log
	}
	c, ok := ctx.(*gin.Context)
	if c == nil || !ok {
		return log
	}

	traceID := c.DefaultQuery("trace_id", "")
	if len(traceID) == 0 {
		traceID = c.GetString("trace_id")
	}
	startTime := time.Now()
	endTime := time.Now()
	latencyTime := endTime.Sub(startTime)
	reqMethod := c.Request.Method
	reqURI := c.Request.RequestURI
	proto := c.Request.Proto
	contentType := c.ContentType()
	statusCode := c.Writer.Status()
	clientIP := c.ClientIP()
	errMsg := c.Errors
	return fmt.Sprintf("| %s | %d | %v | %s | %s | %s | %s | %s | %s | %s ",
		traceID,
		statusCode,
		latencyTime,
		clientIP,
		reqMethod,
		contentType,
		proto,
		reqURI,
		errMsg,
		log,
	)
}

func NewGinLogFormatter() Formatter {
	return &ginLogFormatter{}
}
