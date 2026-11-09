package extension

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestWriteKernelExtensionErrorClassifiesInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	err := fmt.Errorf("kernel: get installation: %w", domain.ErrInvalidExtensionID)
	engine := gin.New()
	engine.POST("/enable", func(c *gin.Context) { writeKernelExtensionError(c, err) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/enable", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
