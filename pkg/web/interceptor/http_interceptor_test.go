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

package interceptor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/stretchr/testify/assert"
)

func TestHTTPServerLoggingInterceptor(t *testing.T) {
	engine := gin.New()

	var logger nlog.NLog = *nlog.DefaultLogger()

	engine.Use(HTTPServerLoggingInterceptor(logger))
	engine.GET("/logging-test", func(c *gin.Context) {
		c.String(200, "OK")
	})

	t.Run("should_log_request_details", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/logging-test", nil)
		req.Header.Set(constants.XForwardHostHeader, "test-host")

		engine.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestHTTPTokenAuthInterceptor(t *testing.T) {
	validToken := "test-token-123"

	tests := []struct {
		name        string
		tokenHeader string
		wantStatus  int
	}{
		{
			name:        "valid token",
			tokenHeader: validToken,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "missing token",
			tokenHeader: "",
			wantStatus:  http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(HTTPTokenAuthInterceptor(validToken))
			engine.GET("/auth-test", func(c *gin.Context) {
				c.String(200, "OK")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/auth-test", nil)
			if tt.tokenHeader != "" {
				req.Header.Set(constants.TokenHeader, tt.tokenHeader)
			}

			engine.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHTTPSourceAuthInterceptor(t *testing.T) {
	t.Run("valid source header", func(t *testing.T) {
		engine := gin.New()
		engine.Use(HTTPSourceAuthInterceptor())
		engine.GET("/source-test", func(c *gin.Context) {
			source := c.GetString(constants.SourceDomainKey)
			assert.Equal(t, "alice", source)
			c.String(200, "OK")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/source-test", nil)
		req.Header.Set(constants.SourceDomainHeader, "alice")

		engine.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("missing source header", func(t *testing.T) {
		engine := gin.New()
		engine.Use(HTTPSourceAuthInterceptor())
		engine.GET("/source-test", func(c *gin.Context) {
			c.String(200, "OK")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/source-test", nil)

		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHTTPSetMasterRoleInterceptor(t *testing.T) {
	engine := gin.New()
	engine.Use(HTTPSetMasterRoleInterceptor())
	engine.GET("/master-test", func(c *gin.Context) {
		role := c.GetString(constants.AuthRole)
		assert.Equal(t, constants.AuthRoleMaster, role)
		c.String(200, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/master-test", nil)

	engine.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
