package extension

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

func TestStandaloneMigrationRequiresVerifiedPackage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewUpdateAPI(nil).RegisterRoutes(router.Group("/api/extensions"))
	request := httptest.NewRequest(http.MethodPost, "/api/extensions/migrations/execute", strings.NewReader(`{"extensionId":"example","fromVersion":"1.0.0","toVersion":"2.0.0"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "MIGRATION_PACKAGE_CONTEXT_REQUIRED") {
		t.Fatalf("expected a package-context error, got %d: %s", response.Code, response.Body.String())
	}
}

func TestPackageOperationViewPreservesFailureDetails(t *testing.T) {
	view := kernelPackageOperationView(kernelruntime.PackageOperationRecord{ErrorCode: "failed", ErrorDetail: "original operation cause"}, []kernelruntime.PackageOperationStep{{ErrorCode: "step_failed", ErrorDetail: "original step cause"}})
	if view.ErrorDetail != "original operation cause" || len(view.Steps) != 1 || view.Steps[0].ErrorDetail != "original step cause" {
		t.Fatalf("failure details missing: %+v", view)
	}
}

func TestLegacyRecoveryRequiresPackageContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewUpdateAPI(nil).RegisterRoutes(router.Group("/api/extensions"))
	for _, path := range []string{"/api/extensions/rollbacks/old/execute", "/api/extensions/rollbacks/old/recover", "/api/extensions/recovery/execute"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "PACKAGE_RECOVERY_CONTEXT_REQUIRED") {
			t.Fatalf("%s: expected package recovery error, got %d: %s", path, response.Code, response.Body.String())
		}
	}
}
