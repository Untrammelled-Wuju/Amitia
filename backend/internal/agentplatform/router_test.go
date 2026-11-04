package agentplatform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoleTeamRouterDoesNotExposeLegacySurfaces(t *testing.T) {
	service, spaceID, ownerID := newTestPlatform(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api"), service, spaceID, ownerID)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/roles/teams", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("team route status = %d, body = %s", response.Code, response.Body.String())
	}

	for _, path := range []string{
		"/api/agent-platform/auth/login",
		"/api/roles/evaluations/suites",
		"/api/roles/activity",
		"/api/roles/a2a/cards",
		"/api/public/a2a/tasks",
	} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}
}
