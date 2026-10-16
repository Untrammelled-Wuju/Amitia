package workbenchbrowser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/browser"
)

func TestIndependentBrowserRoutesRequireAuthorizedDevice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(browser.NewDisabledProvider())
	router := gin.New()
	h.RegisterRoutes(router.Group("/api"))
	base := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, IsLocalTrusted: true, SpaceID: "space", DeviceID: "device", Permissions: []string{auth.PermWorkbenchBrowserControl}}
	for _, scenario := range []struct {
		name   string
		mutate func(*auth.ActorContext)
	}{
		{"extension", func(a *auth.ActorContext) { a.PrincipalType = auth.PrincipalExtension }},
		{"worker", func(a *auth.ActorContext) { a.PrincipalType = auth.PrincipalSystemWorker }},
		{"untrusted-ui", func(a *auth.ActorContext) { a.IsLocalTrusted = false }},
		{"without-permission", func(a *auth.ActorContext) { a.Permissions = nil }},
		{"without-device", func(a *auth.ActorContext) { a.DeviceID = "" }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			actor := base.Clone()
			scenario.mutate(actor)
			for _, route := range []struct{ method, path string }{{"POST", "open"}, {"POST", "observe"}, {"GET", "conv/tab"}, {"GET", "conv/tab/screenshot"}, {"GET", "conv/tab/dom"}, {"POST", "conv/tab/navigate"}, {"POST", "conv/tab/input"}, {"POST", "conv/tab/control"}, {"DELETE", "conv/tab"}} {
				request := httptest.NewRequest(route.method, "/api/workbench/browser/"+route.path, strings.NewReader(`{}`))
				request = request.WithContext(auth.WithActor(request.Context(), actor))
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != http.StatusForbidden {
					t.Fatalf("%s %s bypassed permission: %d", route.method, route.path, response.Code)
				}
			}
		})
	}
	for _, actor := range []*auth.ActorContext{base, {PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space", DeviceID: "paired", Permissions: []string{auth.PermWorkbenchBrowserControl}}} {
		request := httptest.NewRequest(http.MethodPost, "/api/workbench/browser/open", strings.NewReader(`{}`))
		request = request.WithContext(auth.WithActor(request.Context(), actor))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("authorized identity did not reach provider: %d", response.Code)
		}
	}
}
