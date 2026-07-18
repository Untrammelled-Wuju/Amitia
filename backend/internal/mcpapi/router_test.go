package mcpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func responseStatus(t *testing.T, err error) int {
	t.Helper()
	recorder := httptest.NewRecorder()
	contextValue, _ := gin.CreateTestContext(recorder)
	respond(contextValue, nil, err)
	return recorder.Code
}

func TestRespondMapsMCPProblemsToHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"record not found", gorm.ErrRecordNotFound, http.StatusNotFound},
		{"permission", errors.New("MCP_TOOL_PERMISSION_DENIED"), http.StatusForbidden},
		{"auth", errors.New("MCP_AUTH_REQUIRED"), http.StatusUnauthorized},
		{"conflict", errors.New("MCP_DEPENDENCY_CONFLICT"), http.StatusConflict},
		{"timeout", errors.New("MCP_PROTOCOL_REQUEST_TIMEOUT"), http.StatusGatewayTimeout},
		{"internal", errors.New("database unavailable"), http.StatusInternalServerError},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if status := responseStatus(t, test.err); status != test.status {
				t.Fatalf("status=%d want=%d", status, test.status)
			}
		})
	}
}
