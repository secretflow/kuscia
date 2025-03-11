package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterMetricsWithDefault(t *testing.T) {
	subsystem := "test"
	metrics, err := registerMetricsWithDefault(subsystem)
	assert.NoError(t, err)
	assert.NotNil(t, metrics.reqCnt)
	assert.NotNil(t, metrics.reqDur)
	assert.NotNil(t, metrics.reqSz)
	assert.NotNil(t, metrics.resSz)
}

func TestRegistGin(t *testing.T) {
	subsystem := "test"
	e := gin.New()
	assert.NoError(t, RegistGin(subsystem, e))
}

func TestRegistGinWithRouter(t *testing.T) {
	subsystem := "test"
	e := gin.New()
	metricsPath := "/metrics"
	assert.NoError(t, RegistGinWithRouter(subsystem, e, metricsPath))

	req := httptest.NewRequest(http.MethodGet, metricsPath, nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegistGinWithRouterAuth(t *testing.T) {
	subsystem := "test"
	e := gin.New()
	metricsPath := "/metrics"
	accounts := gin.Accounts{
		"admin": "password",
	}
	assert.NoError(t, RegistGinWithRouterAuth(subsystem, e, metricsPath, accounts))

	req := httptest.NewRequest(http.MethodGet, metricsPath, nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	req = httptest.NewRequest(http.MethodGet, metricsPath, nil)
	req.SetBasicAuth("admin", "password")
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlerFunc(t *testing.T) {
	subsystem := "test"
	metrics, err := registerMetricsWithDefault(subsystem)
	assert.NoError(t, err)

	e := gin.New()
	e.Use(handlerFunc(metrics, ""))

	e.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	e.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name     string
		give     string
		wantCode int
	}{
		{
			name:     "GET request",
			give:     "/test",
			wantCode: http.StatusOK,
		},
		{
			name:     "POST request",
			give:     "/test",
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.give, nil)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestPrometheusHandler(t *testing.T) {
	e := gin.New()
	e.GET("/metrics", prometheusHandler())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
