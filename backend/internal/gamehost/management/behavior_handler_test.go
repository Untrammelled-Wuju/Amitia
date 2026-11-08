package management

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

func behaviorHandlerTestRouter(service *GameBehaviorService, actor *auth.ActorContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if actor != nil {
			c.Set("actorContext", actor)
		}
		c.Next()
	})
	RegisterGameBehaviorRouter(router.Group("/api"), service)
	return router
}

func TestGameBehaviorInvokeRequiresDeviceIdentity(t *testing.T) {
	router := behaviorHandlerTestRouter(NewGameBehaviorService(nil, nil, nil), nil)
	request := httptest.NewRequest(http.MethodPost, "/api/game-center/behaviors/invoke", bytes.NewBufferString(`{"pluginId":"game","behaviorId":"pause","expectedCatalogRevision":"r1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGameBehaviorInvokePreservesAuthorizationError(t *testing.T) {
	original := errors.New("game mode owner device-a is reconciling state")
	service := NewGameBehaviorService(nil, nil, nil, func(ctx context.Context, spaceID, targetDeviceID, pluginID, behaviorID string) error {
		if spaceID != "space" || targetDeviceID != "device-a" || pluginID != "game" || behaviorID != "pause" {
			t.Fatalf("unexpected authorization input: %s %s %s %s", spaceID, targetDeviceID, pluginID, behaviorID)
		}
		return original
	})
	router := behaviorHandlerTestRouter(service, &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       "space",
		DeviceID:      "device-a",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/game-center/behaviors/invoke", bytes.NewBufferString(`{"pluginId":"game","behaviorId":"pause","expectedCatalogRevision":"r1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !bytes.Contains(response.Body.Bytes(), []byte(original.Error())) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
