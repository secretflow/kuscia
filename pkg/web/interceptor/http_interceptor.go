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
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/web/constants"
)

func HTTPTokenAuthInterceptor(tokenData string) func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader(constants.TokenHeader)
		err := tokenCheck([]string{token}, tokenData)
		if err != nil {
			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
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
