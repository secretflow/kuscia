package interceptor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/stretchr/testify/assert"
)

func TestHttpInterceptor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockWriter := &MockLogWriter{}
	nlog.Setup(nlog.WithLogWriter(mockWriter))
	logger := nlog.GetLogger()

	router.Use(HTTPServerLoggingInterceptor(logger))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	reqBody := []byte(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	assert.Contains(t, mockWriter.logs, "[INFO] POST /test")
}

func TestHttpTokenAuthInterceptor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(HTTPTokenAuthInterceptor("valid-token"))

	router.GET("/secure", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "authorized"})
	})

	req := httptest.NewRequest("GET", "/secure", nil)
	req.Header.Set(constants.TokenHeader, "valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	reqInvalid := httptest.NewRequest("GET", "/secure", nil)
	reqInvalid.Header.Set(constants.TokenHeader, "invalid-token")
	respInvalid := httptest.NewRecorder()
	router.ServeHTTP(respInvalid, reqInvalid)
	assert.Equal(t, http.StatusUnauthorized, respInvalid.Code)
}

func TestHttpSetMasterRoleInterceptor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(HTTPSetMasterRoleInterceptor())

	router.GET("/role", func(c *gin.Context) {
		role, _ := c.Get(constants.AuthRole)
		c.JSON(http.StatusOK, gin.H{"role": role})
	})

	req := httptest.NewRequest("GET", "/role", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var response map[string]string
	_ = json.Unmarshal(resp.Body.Bytes(), &response)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, constants.AuthRoleMaster, response["role"])
}

func TestHttpSourceAuthInterceptor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(HTTPSourceAuthInterceptor())

	router.GET("/source", func(c *gin.Context) {
		source, _ := c.Get(constants.SourceDomainKey)
		c.JSON(http.StatusOK, gin.H{"source": source})
	})

	req := httptest.NewRequest("GET", "/source", nil)
	req.Header.Set(constants.SourceDomainHeader, "domain1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var response map[string]string
	_ = json.Unmarshal(resp.Body.Bytes(), &response)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "domain1", response["source"])

	reqInvalid := httptest.NewRequest("GET", "/source", nil)
	respInvalid := httptest.NewRecorder()
	router.ServeHTTP(respInvalid, reqInvalid)

	assert.Equal(t, http.StatusUnauthorized, respInvalid.Code)
}

// mock
type MockLogWriter struct {
	logs []string
}

func (m *MockLogWriter) log(msg string) {
	m.logs = append(m.logs, msg)
}

func (m *MockLogWriter) Info(msg string)  { m.log("[INFO] " + msg) }
func (m *MockLogWriter) Debug(msg string) { m.log("[DEBUG] " + msg) }
func (m *MockLogWriter) Warn(msg string)  { m.log("[WARN] " + msg) }
func (m *MockLogWriter) Error(msg string) { m.log("[ERROR] " + msg) }
func (m *MockLogWriter) Fatal(msg string) { m.log("[FATAL] " + msg) }
func (m *MockLogWriter) Write(p []byte) (int, error) {
	m.log(string(p))
	return len(p), nil
}
