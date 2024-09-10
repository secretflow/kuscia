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
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var sensitiveFields = []string{"password", "access_key_id", "access_key_secret"}

var sensitiveHTTPPathPrefix = map[string]struct{}{
	"/api/v1/configmanager/config": {},
	"/api/v1/config":               {},
}

var sensitiveGRPCPathPrefix = map[string]struct{}{
	"/kuscia.proto.api.v1alpha1.kusciaapi.ConfigService":   {},
	"/kuscia.proto.api.v1alpha1.confmanager.ConfigService": {},
}

const (
	protocolGRPC = "GRPC"
	protocolHTTP = "HTTP"
	maxSizeBytes = 1024
)

func hasSensitiveHTTPPathPrefix(path string) bool {
	for key := range sensitiveHTTPPathPrefix {
		if strings.Contains(path, key) {
			return true
		}
	}
	return false
}

func hasSensitiveGRPCPathPrefix(path string) bool {
	for key := range sensitiveGRPCPathPrefix {
		if strings.Contains(path, key) {
			return true
		}
	}
	return false
}

func tokenCheck(src []string, target string) error {
	if src == nil || len(src) == 0 {
		return status.Errorf(codes.Unauthenticated, "s not found")
	}
	for _, s := range src {
		if s == target {
			return nil
		}
	}
	return status.Errorf(codes.Unauthenticated, "s unauthorized")
}

func safeLog(Logger nlog.NLog, protocol string, logContextFunc func()) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Errorf("Exception occurred while logging. protocol: [%s]: %v", protocol, err)
		}
	}()
	logContextFunc()
}

func cutSlice(slice []byte, maxLen int) []byte {
	if len(slice) > maxLen {
		return slice[:maxLen]
	}
	return slice
}

type loggerContext struct {
	Protocol     string
	XForwardHost string
	ContextType  string
	Duration     time.Duration

	Errs []error

	RequestPath   string
	RequestMethod string
	RequestBody   []byte

	StatusCode   int
	ResponseBody []byte
}

func printfLoggerContext(logger nlog.NLog, loggerContext *loggerContext) {

	if len(loggerContext.Errs) > 0 {
		logger.Errorf("[%s] [%s] Duration: %s, StatusCode: %d, ForwardHost: [%s], ContextType: [%s], Request: %s, Response: %s, Error: %v",
			loggerContext.Protocol,
			loggerContext.RequestPath,
			loggerContext.Duration,
			loggerContext.StatusCode,
			loggerContext.XForwardHost,
			loggerContext.ContextType,
			cutSlice(loggerContext.RequestBody, maxSizeBytes),
			cutSlice(loggerContext.ResponseBody, maxSizeBytes),
			loggerContext.Errs)
	} else {
		logger.Infof("[%s] [%s %s] Duration: %s, StatusCode: %d, ForwardHost: [%s], ContextType: [%s], Request: %s, Response: %s",
			loggerContext.Protocol,
			loggerContext.RequestMethod,
			loggerContext.RequestPath,
			loggerContext.Duration,
			loggerContext.StatusCode,
			loggerContext.XForwardHost,
			loggerContext.ContextType,
			cutSlice(loggerContext.RequestBody, maxSizeBytes),
			cutSlice(loggerContext.ResponseBody, maxSizeBytes),
		)
	}
}
