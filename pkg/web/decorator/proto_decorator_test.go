package decorator

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/stretchr/testify/assert"
)

// mockHandler implements api.ProtoHandler for testing purposes.
type mockHandler struct{}

func (m *mockHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(api.AnyStringProto{}), reflect.TypeOf(api.AnyStringProto{})
}

func (m *mockHandler) Validate(ctx *api.BizContext, req api.ProtoRequest, errs *errorcode.Errs) {
	if req.(*api.AnyStringProto).Content == "invalid" {
		errs.AppendErr(errors.New("Invalid request"))
	}
}

func (m *mockHandler) Handle(ctx *api.BizContext, req api.ProtoRequest) api.ProtoResponse {
	return &api.AnyStringProto{Content: "Success"}
}

// setupTestRouter initializes a gin.Engine with the ProtoDecorator using the provided options.
func setupTestRouter(options *ProtoDecoratorOptions) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &mockHandler{}
	r.POST("/test", ProtoDecorator(nil, handler, options))
	return r
}

func TestProtoDecorator(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        string
		expectedStatusCode int
		expectedResponse   string
	}{
		{"Valid Request", `{"Content":"valid"}`, http.StatusOK, `{"Content":"Success"}`},
		{"Invalid Request", `{"Content":"invalid"}`, http.StatusOK, `{"status":{"code":400,"message":"Invalid request"}}`},
	}

	// Initialize the router with custom options for the ProtoDecorator.
	r := setupTestRouter(&ProtoDecoratorOptions{
		ValidateFailedHandler: func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse {
			resp := map[string]map[string]interface{}{
				"status": {"code": 400, "message": errs.AppendErr},
			}
			bytes, _ := json.Marshal(resp)
			return &api.AnyStringProto{Content: string(bytes)}
		},
		UnexpectedErrorHandler: func(flow *BizFlow, errs *errorcode.Errs) api.ProtoResponse {
			return &api.AnyStringProto{Content: `{"status":{"code":500,"message":"Internal Server Error"}}`}
		},
		RenderJSONUseProtoNames: true,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Run tests in parallel.

			req, _ := http.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)
			assert.JSONEq(t, tt.expectedResponse, w.Body.String()) // Assert JSON content.
		})
	}
}
