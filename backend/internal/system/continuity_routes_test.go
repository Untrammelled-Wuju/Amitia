package system

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func TestLegacyContinuityRejectsOrdinaryDeviceBeforeStorage(t *testing.T) {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "source", Permissions: auth.StandardPermissions()})
		c.Next()
	})
	registerContinuityRoutes(engine.Group("/api"), &Handler{})
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/threads"}, {http.MethodPost, "/threads"},
		{http.MethodGet, "/threads/id"}, {http.MethodPatch, "/threads/id"},
		{http.MethodGet, "/threads/id/events"}, {http.MethodGet, "/threads/id/waits"},
		{http.MethodPost, "/threads/id/waits"}, {http.MethodPost, "/threads/id/waits/wait/resolve"},
		{http.MethodPost, "/threads/id/waits/wait/cancel"}, {http.MethodPost, "/signals"},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(request.method, "/api/continuity"+request.path, strings.NewReader(`{}`)))
		if response.Code != http.StatusForbidden {
			t.Fatalf("ordinary device reached legacy Core continuity: %s %s %d", request.method, request.path, response.Code)
		}
	}
}

func TestLegacyContinuityKeepsAdministratorAccessAndCoreIntent(t *testing.T) {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "core", IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()})
		c.Next()
	})
	registerContinuityRoutes(engine.Group("/api"), &Handler{})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/continuity/threads", nil))
	if response.Code == http.StatusForbidden || !strings.Contains(response.Body.String(), "持续事项服务不可用") {
		t.Fatalf("administrator did not reach existing handler: %d %s", response.Code, response.Body.String())
	}
	stale := httptest.NewRequest(http.MethodPost, "/api/continuity/signals", strings.NewReader(`{}`))
	stale.Header.Set("X-Amitia-Expected-Core-ID", "previous-core")
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, stale)
	if response.Code != http.StatusConflict {
		t.Fatalf("old Core intent reached continuity handler: %d", response.Code)
	}
}
