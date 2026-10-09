package extension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDevModeDisabledWorkspaceListReportsAvailability(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "false")
	engine := gin.New()
	api := NewDevModeAPI(nil)
	defer api.Stop()
	api.RegisterRoutes(engine.Group("/api/extensions"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/extensions/dev-mode/workspaces", nil))
	var payload struct {
		Enabled    bool              `json:"enabled"`
		Workspaces []json.RawMessage `json:"workspaces"`
		Total      int               `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || payload.Enabled || payload.Workspaces == nil || len(payload.Workspaces) != 0 || payload.Total != 0 {
		t.Fatalf("unexpected availability response: %d %s", response.Code, response.Body.String())
	}
}

func TestDevModeDisabledStillRejectsOperations(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "false")
	api := NewDevModeAPI(nil)
	defer api.Stop()
	engine := gin.New()
	api.RegisterRoutes(engine.Group("/api/extensions"))
	for _, path := range []string{"/workspaces", "/workspaces/test/build", "/workspaces/test/trust", "/workspaces/test/reload"} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/extensions/dev-mode"+path, nil))
		if response.Code != http.StatusForbidden {
			t.Fatalf("disabled operation accepted: %s %d", path, response.Code)
		}
	}
}

func TestDevModeEnabledWithoutKernelReportsServiceUnavailable(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	api := NewDevModeAPI(nil)
	defer api.Stop()
	api.listWorkspaces(ctx)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable kernel hidden: %d", response.Code)
	}
}

func TestDevModeAvailabilityStillRequiresAuthentication(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "false")
	engine := gin.New()
	api := NewDevModeAPI(nil)
	defer api.Stop()
	group := engine.Group("/api/extensions", extensionAuth())
	api.RegisterRoutes(group)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/extensions/dev-mode/workspaces", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("availability bypassed authentication: %d", response.Code)
	}
}
