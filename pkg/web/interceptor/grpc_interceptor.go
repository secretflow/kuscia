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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

// GrpcServerLoggingInterceptor defines the unary interceptor used to log RPC requests made with a unary call.
func GrpcServerLoggingInterceptor(logger nlog.NLog) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		startTime := time.Now()

		defer safeLog(logger, protocolGRPC, func() {
			duration := time.Since(startTime)
			emptyBody := make([]byte, 0)
			var errors []error

			var statusCode = codes.OK
			if err != nil {
				errors = append(errors, err)
				respStatus, _ := status.FromError(err)
				statusCode = respStatus.Code()
			}

			forwardHostsStr := ""
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				forwardHosts := md.Get(constants.XForwardHostHeader)
				forwardHostsStr = strings.Join(forwardHosts, ",")
			} else {
				logger.Warnf("[%s] Get metadata from incoming context failed", protocolGRPC)
			}

			logContext := &loggerContext{
				Protocol:      protocolGRPC,
				RequestMethod: protocolGRPC,
				RequestPath:   info.FullMethod,
				XForwardHost:  forwardHostsStr,
				StatusCode:    int(statusCode),

				Errs:         errors,
				Duration:     duration,
				RequestBody:  emptyBody,
				ResponseBody: emptyBody,
			}

			if !hasSensitiveGRPCPathPrefix(logContext.RequestPath) {
				errors = adjustGRPCRequestAndResponse(logContext, req, resp)
			}
			printfLoggerContext(logger, logContext)
		})
		return handler(ctx, req)
	}
}

func adjustGRPCRequestAndResponse(ctx *loggerContext, req, resp any) (errors []error) {
	if reqByte, reqMarshalErr := json.Marshal(utils.StructToMap(req, sensitiveFields...)); reqMarshalErr != nil {
		errors = append(errors, reqMarshalErr)
	} else {
		ctx.RequestBody = reqByte
	}

	if respByte, respMarshalErr := json.Marshal(utils.StructToMap(resp, sensitiveFields...)); respMarshalErr != nil {
		errors = append(errors, respMarshalErr)
	} else {
		ctx.ResponseBody = respByte
	}
	return errors
}

// GrpcStreamServerLoggingInterceptor defines the stream interceptor used to log RPC requests made with a stream.
func GrpcStreamServerLoggingInterceptor(Logger nlog.NLog) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		Logger.Infof("[%s] Stream [%s] starting", protocolGRPC, info.FullMethod)
		startTime := time.Now()

		wrappedStream := &loggingServerStream{ServerStream: ss}
		var err = handler(srv, wrappedStream)

		defer safeLog(Logger, protocolGRPC, func() {
			duration := time.Since(startTime)

			recvMsgJSON, _ := json.Marshal(utils.StructToMap(wrappedStream.recvMsg, sensitiveFields...))
			sendMsgJSON, _ := json.Marshal(utils.StructToMap(wrappedStream.sendMsg, sensitiveFields...))
			recvMsgJSON = cutSlice(recvMsgJSON, maxSizeBytes)
			sendMsgJSON = cutSlice(sendMsgJSON, maxSizeBytes)
			if err != nil {
				Logger.Errorf("[%s] Stream [%s] finished with error after %s, recvMsg: %s: %v", protocolGRPC, info.FullMethod, duration, recvMsgJSON, err)
			} else {
				Logger.Infof("[%s] Stream [%s] finished successfully after %s. recvMsg: %s,  sendMsg: %s", protocolGRPC, info.FullMethod, duration, recvMsgJSON, sendMsgJSON)
			}
		})
		return err
	}
}

type loggingServerStream struct {
	grpc.ServerStream
	sendMsg any
	recvMsg any
}

// SendMsg Wrapped the method of sending messages to record the send messages.
func (s *loggingServerStream) SendMsg(m interface{}) error {
	s.sendMsg = m
	return s.ServerStream.SendMsg(m)
}

// RecvMsg Wrapped the method of receiving messages to record the received messages.
func (s *loggingServerStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	if err == nil || err == io.EOF {
		s.recvMsg = m
	}
	return err
}

func GrpcServerTokenInterceptor(tokenData string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		tokens := metadata.ValueFromIncomingContext(ctx, strings.ToLower(constants.TokenHeader))
		err = tokenCheck(tokens, tokenData)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func GrpcStreamServerTokenInterceptor(tokenData string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		tokens := metadata.ValueFromIncomingContext(ctx, strings.ToLower(constants.TokenHeader))
		err := tokenCheck(tokens, tokenData)
		if err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func GrpcServerMasterRoleInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx = context.WithValue(ctx, constants.AuthRole, constants.AuthRoleMaster)
		ctx = context.WithValue(ctx, constants.SourceDomainKey, constants.AuthRoleMaster)
		return handler(ctx, req)
	}
}

func GrpcStreamServerMasterRoleInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		ctx = context.WithValue(ctx, constants.AuthRole, constants.AuthRoleMaster)
		ctx = context.WithValue(ctx, constants.SourceDomainKey, constants.AuthRoleMaster)
		return handler(srv, ss)
	}
}

func GrpcClientTokenInterceptor(tokenData string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, strings.ToLower(constants.TokenHeader), tokenData)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryRecoverInterceptor returns a new unary server interceptors that recovers from panics.
func UnaryRecoverInterceptor(errorCode pberrorcode.ErrorCode) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				nlog.Errorf("[%s] Recovered from panic: %+v, stack: %s", info.FullMethod, r, debug.Stack())
				wrappedErr := fmt.Errorf("%+v", r)
				resp = &v1alpha1.ErrorResponse{
					Status: &v1alpha1.Status{
						Code:    int32(errorCode),
						Message: wrappedErr.Error(),
					},
				}
			}
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}

// StreamRecoverInterceptor returns a new stream server interceptors that recovers from panics.
func StreamRecoverInterceptor(errorCode pberrorcode.ErrorCode) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				nlog.Errorf("[%s] Recovered from panic: %+v, stack: %s", info.FullMethod, r, debug.Stack())
				wrappedErr := fmt.Errorf("%+v", r)
				resp := &v1alpha1.ErrorResponse{
					Status: &v1alpha1.Status{
						Code:    int32(errorCode),
						Message: wrappedErr.Error(),
					},
				}
				_ = ss.SendMsg(resp)
			}
		}()
		err = handler(srv, ss)
		return err
	}
}
