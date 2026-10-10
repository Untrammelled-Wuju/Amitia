package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func TestLegacyChatRoutesRejectOrdinaryDeviceBeforeCoreStorage(t *testing.T) {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "source", Permissions: auth.StandardPermissions()})
		c.Next()
	})
	registerChatRoutes(engine.Group("/api"), &Handler{})
	checked := 0
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/chats/") && route.Path != "/api/chat" {
			continue
		}
		path := strings.ReplaceAll(route.Path, ":id", "core-private-conversation")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(route.Method, path, strings.NewReader(`{}`)))
		if response.Code != http.StatusForbidden {
			t.Fatalf("ordinary device reached legacy chat: %s %s %d", route.Method, path, response.Code)
		}
		checked++
	}
	if checked < 19 {
		t.Fatalf("legacy chat registration incomplete: %d", checked)
	}
}

func TestLegacyChatAdministratorKeepsHandlerAndOriginalCore(t *testing.T) {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "core", IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()})
		c.Next()
	})
	registerChatRoutes(engine.Group("/api"), &Handler{})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/chats/conversations", strings.NewReader(`{`)))
	if response.Code == http.StatusForbidden || !strings.Contains(response.Body.String(), "unexpected EOF") {
		t.Fatalf("administrator did not reach chat handler: %d %s", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/chats/all", nil)
	request.Header.Set("X-Amitia-Expected-Core-ID", "previous-core")
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("old Core chat intent accepted: %d", response.Code)
	}
}
