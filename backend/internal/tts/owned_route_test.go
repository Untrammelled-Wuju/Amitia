package tts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/app"
)

func TestBoundAdministratorCannotBypassOwnedSpeechThroughLegacyRoutes(t *testing.T) {
	for _, path := range []string{"/api/tts/synthesize", "/api/tts/preview", "/api/tts/play/owned-message"} {
		router := gin.New()
		group := router.Group("/api", func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, Permissions: auth.OwnerDevicePermissions()})
			c.Next()
		})
		RegisterTtsRouter(group, &app.AppContext{})
		method := http.MethodPost
		if strings.Contains(path, "/play/") {
			method = http.MethodGet
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(`{"text":"不应调用模型"}`)))
		if recorder.Code != 409 || !strings.Contains(recorder.Body.String(), "mesh.speech_scope_required") {
			t.Fatalf("legacy speech bypass accepted: %s %d", path, recorder.Code)
		}
	}
}

func TestOrdinaryBoundDeviceCannotCreateOrDeleteCoreVoiceClone(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		router := gin.New()
		group := router.Group("/api", func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, Permissions: auth.StandardPermissions()})
			c.Next()
		})
		RegisterTtsRouter(group, &app.AppContext{})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(method, "/api/tts/voice-clone", nil))
		if recorder.Code != 403 {
			t.Fatalf("ordinary device reached paid clone mutation: %s %d", method, recorder.Code)
		}
	}
}
