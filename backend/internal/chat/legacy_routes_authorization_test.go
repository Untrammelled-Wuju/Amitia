package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func TestLegacyChatRoutesAreRemovedForEveryPrincipal(t *testing.T) {
	for _, actor := range []*auth.ActorContext{{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "source", Permissions: auth.StandardPermissions()}, {PrincipalType: auth.PrincipalLocalUI, SpaceID: "core", DeviceID: "core", IsLocalTrusted: true, Permissions: auth.OwnerDevicePermissions()}} {
		engine := gin.New()
		engine.Use(func(c *gin.Context) { c.Set("actorContext", actor); c.Next() })
		RegisterModelRouter(engine.Group("/api"), nil)
		for _, route := range []struct{ method, path string }{{"POST", "/api/chat"}, {"GET", "/api/chats/stats"}, {"POST", "/api/chats/conversations"}, {"DELETE", "/api/chats/all"}, {"POST", "/api/chats/cleanup/confirm"}, {"GET", "/api/chats/conversations/existing/messages"}} {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
			if response.Code != http.StatusNotFound {
				t.Fatalf("removed chat route still reachable: %s %s %d", route.method, route.path, response.Code)
			}
		}
		modelRoutes := 0
		for _, route := range engine.Routes() {
			if strings.HasPrefix(route.Path, "/api/chats/") || route.Path == "/api/chat" {
				t.Fatal("legacy chat route registered")
			}
			if strings.HasPrefix(route.Path, "/api/model/") {
				modelRoutes++
			}
		}
		if modelRoutes < 15 {
			t.Fatalf("model management routes lost: %d", modelRoutes)
		}
	}
}
