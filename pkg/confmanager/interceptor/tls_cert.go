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
	"crypto/x509/pkix"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

var grpcTLSCertKey struct{}

// TLSCertFromGRPCContext get tls cert info from grpc context.
func TLSCertFromGRPCContext(ctx context.Context) (p *pkix.Name) {
	p, ok := ctx.Value(grpcTLSCertKey).(*pkix.Name)
	if !ok {
		return nil
	}
	return p
}

func newTLSCertContext(ctx context.Context, tlsCert *pkix.Name) context.Context {
	if tlsCert != nil {
		return context.WithValue(ctx, grpcTLSCertKey, tlsCert)
	}
	return ctx
}

// GRPCTLSCertInfoInterceptor is an interceptor for grpc.It catches tls cert info from grpc context.Context, and you can get tls cert info by TLSCertFromGRPCContext.
func GRPCTLSCertInfoInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	certInfoFromContext := func(ctx context.Context) *pkix.Name {
		if p, ok := peer.FromContext(ctx); ok {
			if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
				if len(tlsInfo.State.VerifiedChains) > 0 && len(tlsInfo.State.VerifiedChains[0]) > 0 {
					return &tlsInfo.State.VerifiedChains[0][0].Subject
				}
			}
		}
		return nil
	}

	return handler(newTLSCertContext(ctx, certInfoFromContext(ctx)), req)
}

const (
	httpTLSCertKey = "TLSCert"
)

// TLSCertFromGinContext get tls cert info from gin context.
func TLSCertFromGinContext(c *gin.Context) *pkix.Name {
	cert, exist := c.Get(httpTLSCertKey)
	if !exist {
		return nil
	}
	certPkix := cert.(pkix.Name)
	return &certPkix
}

// HTTPTLSCertInfoInterceptor is an interceptor for gin.It catches tls cert info from gin.Context, and you can get tls cert info by TLSCertFromGinContext.
func HTTPTLSCertInfoInterceptor(c *gin.Context) {
	if c.Request.TLS != nil && len(c.Request.TLS.VerifiedChains) > 0 && len(c.Request.TLS.VerifiedChains[0]) > 0 {
		c.Set(httpTLSCertKey, c.Request.TLS.VerifiedChains[0][0].Subject)
	}
	c.Next()
}
