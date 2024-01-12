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
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/secretflow/kuscia/pkg/web/constants"
)

func GrpcServerTokenInterceptor(tokenData string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		tokens := metadata.ValueFromIncomingContext(ctx, strings.ToLower(constants.TokenHeader))
		err = tokenCheck(tokens, tokenData)
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, constants.AuthRole, constants.AuthRoleMaster)
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
		ctx = context.WithValue(ctx, constants.AuthRole, constants.AuthRoleMaster)
		return handler(srv, ss)
	}
}

func GrpcClientTokenInterceptor(tokenData string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, strings.ToLower(constants.TokenHeader), tokenData)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
